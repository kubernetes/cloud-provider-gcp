//go:build !providerless
// +build !providerless

/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package ipam

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/google/go-cmp/cmp"
	"k8s.io/klog/v2"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	informers "k8s.io/client-go/informers/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"

	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	cloudprovider "k8s.io/cloud-provider"
	networkv1 "k8s.io/cloud-provider-gcp/crd/apis/network/v1"
	networkinformer "k8s.io/cloud-provider-gcp/crd/client/network/informers/externalversions/network/v1"
	alphanetworkinformer "k8s.io/cloud-provider-gcp/crd/client/network/informers/externalversions/network/v1alpha1"
	networklister "k8s.io/cloud-provider-gcp/crd/client/network/listers/network/v1"
	alphanetworklister "k8s.io/cloud-provider-gcp/crd/client/network/listers/network/v1alpha1"
	nodeutil "k8s.io/cloud-provider-gcp/pkg/util"
	utilnode "k8s.io/cloud-provider-gcp/pkg/util/node"
	utiltaints "k8s.io/cloud-provider-gcp/pkg/util/taints"
	"k8s.io/cloud-provider-gcp/providers/gce"
	netutils "k8s.io/utils/net"
)

// nodeProcessingInfo tracks information related to current nodes in processing
type nodeProcessingInfo struct {
	retries int
}

// cloudCIDRAllocator allocates node CIDRs according to IP address aliases
// assigned by the cloud provider. In this case, the allocation and
// deallocation is delegated to the external provider, and the controller
// merely takes the assignment and updates the node spec.
type cloudCIDRAllocator struct {
	client clientset.Interface
	cloud  *gce.Cloud
	// networksLister is able to list/get networks and is populated by the shared network informer passed to
	// NewCloudCIDRAllocator.
	networksLister networklister.NetworkLister
	// gnpLister is able to list/get GKENetworkParamSet and is populated by the shared GKENewtorkParamSet informer passed to
	// NewCloudCIDRAllocator.
	gnpLister alphanetworklister.GKENetworkParamSetLister
	// nodeLister is able to list/get nodes and is populated by the shared informer passed to
	// NewCloudCIDRAllocator.
	nodeLister corelisters.NodeLister
	// nodesSynced returns true if the node shared informer has been synced at least once.
	nodesSynced cache.InformerSynced

	// Channel that is used to pass updating Nodes to the background.
	// This increases the throughput of CIDR assignment by parallelization
	// and not blocking on long operations (which shouldn't be done from
	// event handlers anyway).
	nodeUpdateChannel chan string
	recorder          record.EventRecorder

	// Keep a set of nodes that are currectly being processed to avoid races in CIDR allocation
	lock              sync.Mutex
	nodesInProcessing map[string]*nodeProcessingInfo
}

var _ CIDRAllocator = (*cloudCIDRAllocator)(nil)

// NewCloudCIDRAllocator creates a new cloud CIDR allocator.
func NewCloudCIDRAllocator(client clientset.Interface, cloud cloudprovider.Interface, nwInformer networkinformer.NetworkInformer, gnpInformer alphanetworkinformer.GKENetworkParamSetInformer, nodeInformer informers.NodeInformer) (CIDRAllocator, error) {
	if client == nil {
		klog.Fatalf("kubeClient is nil when starting NodeController")
	}

	eventBroadcaster := record.NewBroadcaster()
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, v1.EventSource{Component: "cidrAllocator"})
	eventBroadcaster.StartStructuredLogging(0)
	klog.V(0).Infof("Sending events to api server.")
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: client.CoreV1().Events("")})

	gceCloud, ok := cloud.(*gce.Cloud)
	if !ok {
		err := fmt.Errorf("cloudCIDRAllocator does not support %v provider", cloud.ProviderName())
		return nil, err
	}
	ca := &cloudCIDRAllocator{
		client:            client,
		cloud:             gceCloud,
		networksLister:    nwInformer.Lister(),
		gnpLister:         gnpInformer.Lister(),
		nodeLister:        nodeInformer.Lister(),
		nodesSynced:       nodeInformer.Informer().HasSynced,
		nodeUpdateChannel: make(chan string, cidrUpdateQueueSize),
		recorder:          recorder,
		nodesInProcessing: map[string]*nodeProcessingInfo{},
	}

	nodeInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: nodeutil.CreateAddNodeHandler(ca.AllocateOrOccupyCIDR),
		UpdateFunc: nodeutil.CreateUpdateNodeHandler(func(_, newNode *v1.Node) error {
			if newNode.Spec.PodCIDR == "" {
				return ca.AllocateOrOccupyCIDR(newNode)
			}
			// Even if PodCIDR is assigned, but NetworkUnavailable condition is
			// set to true, we need to process the node to set the condition.
			networkUnavailableTaint := &v1.Taint{Key: v1.TaintNodeNetworkUnavailable, Effect: v1.TaintEffectNoSchedule}
			_, cond := nodeutil.GetNodeCondition(&newNode.Status, v1.NodeNetworkUnavailable)
			if cond == nil || cond.Status != v1.ConditionFalse || utiltaints.TaintExists(newNode.Spec.Taints, networkUnavailableTaint) {
				return ca.AllocateOrOccupyCIDR(newNode)
			}
			return nil
		}),
		DeleteFunc: nodeutil.CreateDeleteNodeHandler(ca.ReleaseCIDR),
	})

	klog.V(0).Infof("Using cloud CIDR allocator (provider: %v)", cloud.ProviderName())
	return ca, nil
}

func (ca *cloudCIDRAllocator) Run(stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()

	klog.Infof("Starting cloud CIDR allocator")
	defer klog.Infof("Shutting down cloud CIDR allocator")

	if !cache.WaitForNamedCacheSync("cidrallocator", stopCh, ca.nodesSynced) {
		return
	}

	for i := 0; i < cidrUpdateWorkers; i++ {
		go ca.worker(stopCh)
	}

	<-stopCh
}

func (ca *cloudCIDRAllocator) worker(stopChan <-chan struct{}) {
	for {
		select {
		case workItem, ok := <-ca.nodeUpdateChannel:
			if !ok {
				klog.Warning("Channel nodeCIDRUpdateChannel was unexpectedly closed")
				return
			}
			if err := ca.updateCIDRAllocation(workItem); err == nil {
				klog.V(3).Infof("Updated CIDR for %q", workItem)
			} else {
				klog.Errorf("Error updating CIDR for %q: %v", workItem, err)
				if canRetry, timeout := ca.retryParams(workItem); canRetry {
					klog.V(2).Infof("Retrying update for %q after %v", workItem, timeout)
					time.AfterFunc(timeout, func() {
						// Requeue the failed node for update again.
						ca.nodeUpdateChannel <- workItem
					})
					continue
				}
				klog.Errorf("Exceeded retry count for %q, dropping from queue", workItem)
			}
			ca.removeNodeFromProcessing(workItem)
		case <-stopChan:
			return
		}
	}
}

func (ca *cloudCIDRAllocator) insertNodeToProcessing(nodeName string) bool {
	ca.lock.Lock()
	defer ca.lock.Unlock()
	if _, found := ca.nodesInProcessing[nodeName]; found {
		return false
	}
	ca.nodesInProcessing[nodeName] = &nodeProcessingInfo{}
	return true
}

func (ca *cloudCIDRAllocator) retryParams(nodeName string) (bool, time.Duration) {
	ca.lock.Lock()
	defer ca.lock.Unlock()

	entry, ok := ca.nodesInProcessing[nodeName]
	if !ok {
		klog.Errorf("Cannot get retryParams for %q as entry does not exist", nodeName)
		return false, 0
	}

	count := entry.retries + 1
	if count > updateMaxRetries {
		return false, 0
	}
	ca.nodesInProcessing[nodeName].retries = count

	return true, nodeUpdateRetryTimeout(count)
}

func nodeUpdateRetryTimeout(count int) time.Duration {
	timeout := updateRetryTimeout
	for i := 0; i < count && timeout < maxUpdateRetryTimeout; i++ {
		timeout *= 2
	}
	if timeout > maxUpdateRetryTimeout {
		timeout = maxUpdateRetryTimeout
	}
	return time.Duration(timeout.Nanoseconds()/2 + rand.Int63n(timeout.Nanoseconds()))
}

func (ca *cloudCIDRAllocator) removeNodeFromProcessing(nodeName string) {
	ca.lock.Lock()
	defer ca.lock.Unlock()
	delete(ca.nodesInProcessing, nodeName)
}

// WARNING: If you're adding any return calls or defer any more work from this
// function you have to make sure to update nodesInProcessing properly with the
// disposition of the node when the work is done.
func (ca *cloudCIDRAllocator) AllocateOrOccupyCIDR(node *v1.Node) error {
	if node == nil {
		return nil
	}
	if !ca.insertNodeToProcessing(node.Name) {
		klog.V(2).InfoS("Node is already in a process of CIDR assignment", "node", klog.KObj(node))
		return nil
	}

	klog.V(4).Infof("Putting node %s into the work queue", node.Name)
	ca.nodeUpdateChannel <- node.Name
	return nil
}

// updateCIDRAllocation assigns CIDR to Node and sends an update to the API server.
func (ca *cloudCIDRAllocator) updateCIDRAllocation(nodeName string) error {
	node, err := ca.nodeLister.Get(nodeName)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil // node no longer available, skip processing
		}
		klog.ErrorS(err, "Failed while getting the node for updating Node.Spec.PodCIDR", "nodeName", nodeName)
		return err
	}
	if node.Spec.ProviderID == "" {
		return fmt.Errorf("node %s doesn't have providerID", nodeName)
	}
	instance, err := ca.cloud.InstanceByProviderID(node.Spec.ProviderID)
	if err != nil {
		nodeutil.RecordNodeStatusChange(ca.recorder, node, "CIDRNotAvailable")
		return fmt.Errorf("failed to get instance from provider: %v", err)
	}

	cidrStrings := make([]string, 0)
	var northInterfaces networkv1.NorthInterfacesAnnotation
	var additionalNodeNetworks networkv1.MultiNetworkAnnotation

	if len(instance.NetworkInterfaces) == 0 || (len(instance.NetworkInterfaces) == 1 && len(instance.NetworkInterfaces[0].AliasIpRanges) == 0) {
		nodeutil.RecordNodeStatusChange(ca.recorder, node, "CIDRNotAvailable")
		return fmt.Errorf("failed to allocate cidr: Node %v has no ranges from which CIDRs can be allocated", node.Name)
	}
	// nodes in clusters WITHOUT multi-networking are expected to have only 1 network-interface with 1 alias IP range.
	if len(instance.NetworkInterfaces) == 1 && len(instance.NetworkInterfaces[0].AliasIpRanges) == 1 {
		cidrStrings = append(cidrStrings, instance.NetworkInterfaces[0].AliasIpRanges[0].IpCidrRange)
		ipv6Addr := ca.cloud.GetIPV6Address(instance.NetworkInterfaces[0])
		if ipv6Addr != nil {
			cidrStrings = append(cidrStrings, ipv6Addr.String())
		}
	} else {
		// multi-networking enabled clusters
		cidrStrings, northInterfaces, additionalNodeNetworks, err = ca.PerformMultiNetworkCIDRAllocation(node, instance.NetworkInterfaces)
		if err != nil {
			nodeutil.RecordNodeStatusChange(ca.recorder, node, "CIDRNotAvailable")
			return fmt.Errorf("failed to get cidr(s) from provider: %v", err)
		}
	}
	if len(cidrStrings) == 0 {
		nodeutil.RecordNodeStatusChange(ca.recorder, node, "CIDRNotAvailable")
		return fmt.Errorf("failed to allocate cidr: Node %v has no CIDRs", node.Name)
	}
	//Can have at most 2 ips (one for v4 and one for v6)
	if len(cidrStrings) > 2 {
		klog.InfoS("Got more than 2 ips, truncating to 2", "cidrStrings", cidrStrings)
		cidrStrings = cidrStrings[:2]
	}

	cidrs, err := netutils.ParseCIDRs(cidrStrings)
	if err != nil {
		return fmt.Errorf("failed to parse strings %v as CIDRs: %v", cidrStrings, err)
	}

	needUpdate, err := needPodCIDRsUpdate(node, cidrs)
	if err != nil {
		return fmt.Errorf("err: %v, CIDRS: %v", err, cidrStrings)
	}
	if needUpdate {
		if node.Spec.PodCIDR != "" {
			klog.ErrorS(nil, "PodCIDR being reassigned!", "nodeName", node.Name, "node.Spec.PodCIDRs", node.Spec.PodCIDRs, "cidrStrings", cidrStrings)
			// We fall through and set the CIDR despite this error. This
			// implements the same logic as implemented in the
			// rangeAllocator.
			//
			// See https://github.com/kubernetes/kubernetes/pull/42147#discussion_r103357248
		}
		for i := 0; i < cidrUpdateRetries; i++ {
			if err = utilnode.PatchNodeCIDRs(ca.client, types.NodeName(node.Name), cidrStrings); err == nil {
				klog.InfoS("Set the node PodCIDRs", "nodeName", node.Name, "cidrStrings", cidrStrings)
				break
			}
		}
	}
	if err != nil {
		nodeutil.RecordNodeStatusChange(ca.recorder, node, "CIDRAssignmentFailed")
		klog.ErrorS(err, "Failed to update the node PodCIDR after multiple attempts", "nodeName", node.Name, "cidrStrings", cidrStrings)
		return err
	}

	if northInterfaces != nil || additionalNodeNetworks != nil {
		if err := ca.updateMultiNetworkAnnotations(node, northInterfaces, additionalNodeNetworks); err != nil {
			return err
		}
	}
	err = utilnode.SetNodeCondition(ca.client, types.NodeName(node.Name), v1.NodeCondition{
		Type:               v1.NodeNetworkUnavailable,
		Status:             v1.ConditionFalse,
		Reason:             "RouteCreated",
		Message:            "NodeController create implicit route",
		LastTransitionTime: metav1.Now(),
	})
	if err != nil {
		klog.ErrorS(err, "Error setting route status for the node", "nodeName", node.Name)
	}
	return err
}

func needPodCIDRsUpdate(node *v1.Node, podCIDRs []*net.IPNet) (bool, error) {
	if node.Spec.PodCIDR == "" {
		return true, nil
	}
	_, nodePodCIDR, err := net.ParseCIDR(node.Spec.PodCIDR)
	if err != nil {
		klog.ErrorS(err, "Found invalid node.Spec.PodCIDR", "node.Spec.PodCIDR", node.Spec.PodCIDR)
		// We will try to overwrite with new CIDR(s)
		return true, nil
	}
	nodePodCIDRs, err := netutils.ParseCIDRs(node.Spec.PodCIDRs)
	if err != nil {
		klog.ErrorS(err, "Found invalid node.Spec.PodCIDRs", "node.Spec.PodCIDRs", node.Spec.PodCIDRs)
		// We will try to overwrite with new CIDR(s)
		return true, nil
	}

	if len(podCIDRs) == 1 {
		if cmp.Equal(nodePodCIDR, podCIDRs[0]) {
			klog.V(4).InfoS("Node already has allocated CIDR. It matches the proposed one.", "nodeName", node.Name, "podCIDRs[0]", podCIDRs[0])
			return false, nil
		}
	} else if len(nodePodCIDRs) == len(podCIDRs) {
		if dualStack, _ := netutils.IsDualStackCIDRs(podCIDRs); !dualStack {
			return false, fmt.Errorf("IPs are not dual stack")
		}
		for idx, cidr := range podCIDRs {
			if !cmp.Equal(nodePodCIDRs[idx], cidr) {
				return true, nil
			}
		}
		klog.V(4).InfoS("Node already has allocated CIDRs. It matches the proposed one.", "nodeName", node.Name, "podCIDRs", podCIDRs)
		return false, nil
	}

	return true, nil
}

func (ca *cloudCIDRAllocator) ReleaseCIDR(node *v1.Node) error {
	klog.V(2).Infof("Node %v PodCIDR (%v) will be released by external cloud provider (not managed by controller)",
		node.Name, node.Spec.PodCIDR)
	return nil
}

func (ca *cloudCIDRAllocator) updateMultiNetworkAnnotations(node *v1.Node, northInterfaces networkv1.NorthInterfacesAnnotation, additionalNodeNetworks networkv1.MultiNetworkAnnotation) error {
	northInterfaceAnn, err := networkv1.MarshalNorthInterfacesAnnotation(northInterfaces)
	if err != nil {
		klog.ErrorS(err, "Failed to marshal the north interfaces annotation for multi-networking", "nodeName", node.Name)
		return err
	}
	additionalNodeNwAnn, err := networkv1.MarshalAnnotation(additionalNodeNetworks)
	if err != nil {
		klog.ErrorS(err, "Failed to marshal the additional node networks annotation for multi-networking", "nodeName", node.Name)
		return err
	}
	if node.Annotations == nil {
		node.Annotations = make(map[string]string)
	}
	node.Annotations[networkv1.NorthInterfacesAnnotationKey] = northInterfaceAnn
	node.Annotations[networkv1.MultiNetworkAnnotationKey] = additionalNodeNwAnn
	node.Status.Capacity, err = allocateIPCapacity(node, additionalNodeNetworks)
	if err != nil {
		return err
	}
	// Prepare patch bytes for the node update.
	patchBytes, err := json.Marshal([]interface{}{
		map[string]interface{}{
			"op":    "replace",
			"path":  "/metadata/annotations",
			"value": node.Annotations,
		},
		map[string]interface{}{
			"op":    "add",
			"path":  "/status/capacity",
			"value": node.Status.Capacity,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to build patch bytes for multi-networking: %v", err)
	}
	// Since dynamic network addition/deletion is a use case to be supported, we aspire to build these annotations and IP capacities every time from scratch.
	// Hence we are using a JSON patch merge strategy instead of strategic merge strategy on the node during update.
	if _, err = ca.client.CoreV1().Nodes().Patch(context.TODO(), node.Name, types.JSONPatchType, patchBytes, metav1.PatchOptions{}, "status"); err != nil {
		nodeutil.RecordNodeStatusChange(ca.recorder, node, "CIDRAssignmentFailed")
		klog.ErrorS(err, "Failed to update the node annotations and capacity for multi-networking", "nodeName", node.Name)
		return err
	}
	return nil
}

// allocateIPCapacity updates the extended IP resource capacity for every non-default network on the node.
func allocateIPCapacity(node *v1.Node, nodeNetworks networkv1.MultiNetworkAnnotation) (v1.ResourceList, error) {
	resourceList := node.Status.Capacity
	if resourceList == nil {
		resourceList = make(v1.ResourceList)
	}
	// Rebuild the IP capacity for all the networks on the node by deleting the existing IP capacities first.
	for name := range resourceList {
		if strings.HasPrefix(name.String(), networkv1.NetworkResourceKeyPrefix) {
			delete(resourceList, name)
		}
	}
	for _, nw := range nodeNetworks {
		_, ipNet, err := net.ParseCIDR(nw.Cidrs[0])
		if err != nil {
			return nil, err
		}
		var ipCount int64 = 1
		size := netutils.RangeSize(ipNet)
		if size > 1 {
			// The number of IPs supported are halved and returned for overprovisioning purposes.
			ipCount = size >> 1
		}
		resourceList[v1.ResourceName(networkv1.NetworkResourceKeyPrefix+nw.Name+".IP")] = *resource.NewQuantity(int64(ipCount), resource.DecimalSI)
	}
	return resourceList, nil
}
