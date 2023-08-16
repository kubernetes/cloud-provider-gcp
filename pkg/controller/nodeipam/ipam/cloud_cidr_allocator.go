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
	"fmt"
	"net"
	"reflect"
	"time"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"
	networkv1 "k8s.io/cloud-provider-gcp/crd/apis/network/v1"
	"k8s.io/klog/v2"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	informers "k8s.io/client-go/informers/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	cloudprovider "k8s.io/cloud-provider"
	networkinformer "k8s.io/cloud-provider-gcp/crd/client/network/informers/externalversions/network/v1"
	networklister "k8s.io/cloud-provider-gcp/crd/client/network/listers/network/v1"
	"k8s.io/cloud-provider-gcp/pkg/controllermetrics"
	nodeutil "k8s.io/cloud-provider-gcp/pkg/util"
	utilnode "k8s.io/cloud-provider-gcp/pkg/util/node"
	utiltaints "k8s.io/cloud-provider-gcp/pkg/util/taints"
	"k8s.io/cloud-provider-gcp/providers/gce"
	v1nodeutil "k8s.io/component-helpers/node/util"
	netutils "k8s.io/utils/net"
)

const workqueueName = "cloudCIDRAllocator"

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
	gnpLister networklister.GKENetworkParamSetLister
	// nodeLister is able to list/get nodes and is populated by the shared informer passed to
	// NewCloudCIDRAllocator.
	nodeLister corelisters.NodeLister
	// nodesSynced returns true if the node shared informer has been synced at least once.
	nodesSynced cache.InformerSynced

	recorder record.EventRecorder
	queue    workqueue.RateLimitingInterface
}

var _ CIDRAllocator = (*cloudCIDRAllocator)(nil)

// NewCloudCIDRAllocator creates a new cloud CIDR allocator.
func NewCloudCIDRAllocator(client clientset.Interface, cloud cloudprovider.Interface, nwInformer networkinformer.NetworkInformer, gnpInformer networkinformer.GKENetworkParamSetInformer, nodeInformer informers.NodeInformer) (CIDRAllocator, error) {
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
		client:         client,
		cloud:          gceCloud,
		networksLister: nwInformer.Lister(),
		gnpLister:      gnpInformer.Lister(),
		nodeLister:     nodeInformer.Lister(),
		nodesSynced:    nodeInformer.Informer().HasSynced,
		recorder:       recorder,
		queue:          workqueue.NewRateLimitingQueueWithConfig(workqueue.DefaultControllerRateLimiter(), workqueue.RateLimitingQueueConfig{Name: workqueueName}),
	}

	nodeInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: nodeutil.CreateAddNodeHandler(ca.AllocateOrOccupyCIDR),
		UpdateFunc: nodeutil.CreateUpdateNodeHandler(func(oldNode, newNode *v1.Node) error {
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

			// Process Node for Multi-Network network-status annotation change
			var oldVal, newVal string
			if newNode.Annotations != nil {
				newVal = newNode.Annotations[networkv1.NodeNetworkAnnotationKey]
			}
			if oldNode.Annotations != nil {
				oldVal = oldNode.Annotations[networkv1.NodeNetworkAnnotationKey]
			}
			if oldVal != newVal {
				return ca.AllocateOrOccupyCIDR(newNode)
			}

			return nil
		}),
		DeleteFunc: nodeutil.CreateDeleteNodeHandler(ca.ReleaseCIDR),
	})

	nwInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(originalObj interface{}) {
			nw, isNetwork := originalObj.(*networkv1.Network)
			if !isNetwork {
				klog.Errorf("Received unexpected object: %v", originalObj)
				return
			}
			if !meta.IsStatusConditionTrue(nw.Status.Conditions, string(networkv1.NetworkConditionStatusReady)) {
				// ignore non-Ready Networks
				klog.V(5).Infof("Ignoring non-Ready Network (%s) create event", nw.Name)
				return
			}
			klog.V(0).Infof("Received Network (%s) create event", nw.Name)
			err := ca.NetworkToNodes(nil)
			if err != nil {
				klog.Errorf("Error while adding Nodes to queue: %v", err)
			}
		},
		UpdateFunc: func(origOldObj, origNewObj interface{}) {
			oldNet := origOldObj.(*networkv1.Network)
			newNet := origNewObj.(*networkv1.Network)
			readyCond := string(networkv1.NetworkConditionStatusReady)
			newStatus := meta.IsStatusConditionTrue(newNet.Status.Conditions, readyCond)
			if meta.IsStatusConditionTrue(oldNet.Status.Conditions, readyCond) != newStatus {
				klog.V(0).Infof("Received Network (%s) update event", newNet.Name)
				var err error
				if newStatus {
					// Networks that Ready condition switched to True, we need to discover
					// it on every node
					err = ca.NetworkToNodes(nil)
				} else {
					// Networks that Ready condition switched to False, we need to remove
					// it only from nodes using it
					err = ca.NetworkToNodes(newNet)
				}
				if err != nil {
					utilruntime.HandleError(fmt.Errorf("error while adding Nodes to queue: %v", err))
				}
			}
		},
		DeleteFunc: func(originalObj interface{}) {
			network, ok := originalObj.(*networkv1.Network)
			if !ok {
				tombstone, ok := originalObj.(cache.DeletedFinalStateUnknown)
				if !ok {
					utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", originalObj))
					return
				}
				network, ok = tombstone.Obj.(*networkv1.Network)
				if !ok {
					utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a Network %#v", originalObj))
					return
				}
			}
			klog.V(0).Infof("Received Network (%s) delete event", network.Name)
			err := ca.NetworkToNodes(network)
			if err != nil {
				klog.Errorf("Error while adding Nodes to queue: %v", err)
			}
		},
	})

	// register Cloud CIDR Allocator metrics
	registerCloudCidrAllocatorMetrics()

	klog.V(0).Infof("Using cloud CIDR allocator (provider: %v)", cloud.ProviderName())
	return ca, nil
}

func (ca *cloudCIDRAllocator) Run(stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()

	ctx, cancelFn := context.WithCancel(context.Background())
	defer cancelFn()
	defer ca.queue.ShutDown()

	klog.Infof("Starting cloud CIDR allocator")
	defer klog.Infof("Shutting down cloud CIDR allocator")

	if !cache.WaitForNamedCacheSync("cidrallocator", stopCh, ca.nodesSynced) {
		return
	}

	for i := 0; i < cidrUpdateWorkers; i++ {
		go wait.UntilWithContext(ctx, ca.runWorker, time.Second)
	}

	<-stopCh
}

func (ca *cloudCIDRAllocator) AllocateOrOccupyCIDR(node *v1.Node) error {
	klog.V(4).Infof("Putting node %s into the work queue", node.Name)
	ca.queue.Add(node.Name)
	return nil
}

func (ca *cloudCIDRAllocator) runWorker(ctx context.Context) {
	for ca.processNextItem(ctx) {
	}
}

func (ca *cloudCIDRAllocator) processNextItem(ctx context.Context) bool {
	key, quit := ca.queue.Get()
	if quit {
		return false
	}

	defer ca.queue.Done(key)

	klog.V(3).Infof("Processing %s", key)
	//TODO: properly enable and pass ctx to updateCIDRAllocation
	err := ca.updateCIDRAllocation(key.(string))
	ca.handleErr(err, key)
	return true
}

// handleErr checks if an error happened and makes sure we will retry later.
func (ca *cloudCIDRAllocator) handleErr(err error, key interface{}) {
	if err == nil {
		// Forget about the #AddRateLimited history of the key on every successful synchronization.
		// This ensures that future processing of updates for this key is not delayed because of
		// an outdated error history.
		ca.queue.Forget(key)
		klog.V(3).Infof("Updated CIDR for %q", key)
		return
	}
	klog.Errorf("Error updating CIDR for %q: %v", key, err)

	// This controller retries updateMaxRetries times if something goes wrong. After that, it stops trying.
	if ca.queue.NumRequeues(key) < updateMaxRetries {
		klog.Warningf("Error while updating Node object, retrying %q: %v", key, err)

		// Re-enqueue the key rate limited. Based on the rate limiter on the
		// queue and the re-enqueue history, the key will be processed later again.
		ca.queue.AddRateLimited(key)
		return
	}

	ca.queue.Forget(key)
	// Report to an external entity that, even after several retries, we could not successfully process this key
	utilruntime.HandleError(err)
	klog.Errorf("Exceeded retry count for %q, dropping from queue", key)
	controllermetrics.WorkqueueDroppedObjects.WithLabelValues(workqueueName).Inc()

}

// updateCIDRAllocation assigns CIDR to Node and sends an update to the API server.
// Operate on the `node` object if any changes have to be done to it in the API.
func (ca *cloudCIDRAllocator) updateCIDRAllocation(nodeName string) error {
	oldNode, err := ca.nodeLister.Get(nodeName)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil // node no longer available, skip processing
		}
		klog.ErrorS(err, "Failed while getting the node for updating Node.Spec.PodCIDR", "nodeName", nodeName)
		return err
	}
	node := oldNode.DeepCopy()

	if node.Spec.ProviderID == "" {
		return fmt.Errorf("node %s doesn't have providerID", nodeName)
	}
	instance, err := ca.cloud.InstanceByProviderID(node.Spec.ProviderID)
	if err != nil {
		nodeutil.RecordNodeStatusChange(ca.recorder, node, "CIDRNotAvailable")
		return fmt.Errorf("failed to get instance from provider: %v", err)
	}

	cidrStrings := make([]string, 0)

	if len(instance.NetworkInterfaces) == 0 || (len(instance.NetworkInterfaces) == 1 && len(instance.NetworkInterfaces[0].AliasIpRanges) == 0) {
		nodeutil.RecordNodeStatusChange(ca.recorder, node, "CIDRNotAvailable")
		return fmt.Errorf("failed to allocate cidr: Node %v has no ranges from which CIDRs can be allocated", node.Name)
	}

	// sets the v1.NodeNetworkUnavailable condition to False
	ca.setNetworkCondition(node)

	// nodes in clusters WITHOUT multi-networking are expected to have only 1 network-interface with 1 alias IP range.
	if len(instance.NetworkInterfaces) == 1 && len(instance.NetworkInterfaces[0].AliasIpRanges) == 1 {
		cidrStrings = append(cidrStrings, instance.NetworkInterfaces[0].AliasIpRanges[0].IpCidrRange)
		ipv6Addr := ca.cloud.GetIPV6Address(instance.NetworkInterfaces[0])
		if ipv6Addr != nil {
			cidrStrings = append(cidrStrings, ipv6Addr.String())
		}
	} else {
		// multi-networking enabled clusters
		cidrStrings, err = ca.performMultiNetworkCIDRAllocation(node, instance.NetworkInterfaces)
		if err != nil {
			nodeutil.RecordNodeStatusChange(ca.recorder, node, "CIDRNotAvailable")
			return fmt.Errorf("failed to get cidr(s) from provider: %v", err)
		}
	}
	if len(cidrStrings) == 0 {
		nodeutil.RecordNodeStatusChange(ca.recorder, node, "CIDRNotAvailable")
		return fmt.Errorf("failed to allocate cidr: Node %v has no CIDRs", node.Name)
	}
	// Can have at most 2 ips (one for v4 and one for v6)
	if len(cidrStrings) > 2 {
		klog.InfoS("Got more than 2 ips, truncating to 2", "cidrStrings", cidrStrings)
		cidrStrings = cidrStrings[:2]
	}

	cidrs, err := netutils.ParseCIDRs(cidrStrings)
	if err != nil {
		return fmt.Errorf("failed to parse strings %v as CIDRs: %v", cidrStrings, err)
	}

	// TODO: revisit need of needPodCIDRsUpdate with current code base
	// additionally: spec.podCIDRs: Forbidden: node updates may not change podCIDR except from "" to valid
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
		node.Spec.PodCIDR = cidrStrings[0]
		node.Spec.PodCIDRs = cidrStrings
	}

	err = ca.updateNodeCIDR(node, oldNode)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(node.Annotations, oldNode.Annotations) {
		// retain old north interfaces annotation
		var oldNorthInterfacesAnnotation networkv1.NorthInterfacesAnnotation
		if ann, exists := oldNode.Annotations[networkv1.NorthInterfacesAnnotationKey]; exists {
			oldNorthInterfacesAnnotation, err = networkv1.ParseNorthInterfacesAnnotation(ann)
			if err != nil {
				klog.ErrorS(err, "Failed to parse north interfaces annotation for multi-networking", "nodeName", oldNode.Name)
			}
		}

		if err = utilnode.PatchNodeMultiNetwork(ca.client, node); err != nil {
			nodeutil.RecordNodeStatusChange(ca.recorder, node, "CIDRAssignmentFailed")
			klog.ErrorS(err, "Failed to update the node annotations and capacity for multi-networking", "nodeName", node.Name)
			return err
		}

		// calculate updates to multinetwork node count metric based on new north interfaces annotation
		if ann, exists := node.Annotations[networkv1.NorthInterfacesAnnotationKey]; exists {
			newNorthInterfacesAnnotation, err := networkv1.ParseNorthInterfacesAnnotation(ann)
			if err != nil {
				klog.ErrorS(err, "Failed to parse north interfaces annotation for multi-networking", "nodeName", node.Name)
			}

			for _, ni := range oldNorthInterfacesAnnotation {
				multiNetworkNodes.WithLabelValues(ni.Network).Dec()
			}
			for _, ni := range newNorthInterfacesAnnotation {
				multiNetworkNodes.WithLabelValues(ni.Network).Inc()
			}
		}
	}
	return err
}

func (ca *cloudCIDRAllocator) setNetworkCondition(node *v1.Node) {
	cond := v1.NodeCondition{
		Type:               v1.NodeNetworkUnavailable,
		Status:             v1.ConditionFalse,
		Reason:             "RouteCreated",
		Message:            "NodeController create implicit route",
		LastTransitionTime: metav1.Now(),
	}
	for i := range node.Status.Conditions {
		if node.Status.Conditions[i].Type == v1.NodeNetworkUnavailable {
			// we do not update Times so that we do not trigger unnecessary updates
			node.Status.Conditions[i].Status = cond.Status
			node.Status.Conditions[i].Reason = cond.Reason
			node.Status.Conditions[i].Message = cond.Message
			return
		}
	}
	// NodeNetworkUnavailable condition not found, lets add it
	node.Status.Conditions = append(node.Status.Conditions, cond)
}

func (ca *cloudCIDRAllocator) updateNodeCIDR(node, oldNode *v1.Node) error {
	var err error

	// update Spec.podCIDR
	if !reflect.DeepEqual(node.Spec, oldNode.Spec) {
		// TODO: remove the retry since it is handled by the reconciliation loop
		for i := 0; i < cidrUpdateRetries; i++ {
			if err = utilnode.PatchNodeCIDRs(ca.client, types.NodeName(node.Name), node.Spec.PodCIDRs); err == nil {
				klog.InfoS("Set the node PodCIDRs", "nodeName", node.Name, "cidrStrings", node.Spec.PodCIDRs)
				break
			}
		}
		if err != nil {
			nodeutil.RecordNodeStatusChange(ca.recorder, node, "CIDRAssignmentFailed")
			klog.ErrorS(err, "Failed to update the node PodCIDR after multiple attempts", "nodeName", node.Name, "cidrStrings", node.Spec.PodCIDRs)
			return err
		}
	}

	// Update Conditions
	if !reflect.DeepEqual(node.Status.Conditions, oldNode.Status.Conditions) {
		_, cond := v1nodeutil.GetNodeCondition(&node.Status, v1.NodeNetworkUnavailable)
		if cond == nil {
			// this should not happen
			return fmt.Errorf("unable to find %s condition in node %s", v1.NodeNetworkUnavailable, node.Name)
		}
		err = utilnode.SetNodeCondition(ca.client, types.NodeName(node.Name), *cond)
		if err != nil {
			klog.ErrorS(err, "Error setting route status for the node", "nodeName", node.Name)
		}
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
