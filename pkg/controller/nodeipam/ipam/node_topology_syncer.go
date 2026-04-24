package ipam

import (
	"context"
	"fmt"
	"strings"

	nodetopologyv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/nodetopology/v1"
	nodetopologyclientset "github.com/GoogleCloudPlatform/gke-networking-api/client/nodetopology/clientset/versioned"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	utilnode "k8s.io/cloud-provider-gcp/pkg/util/node"
	"k8s.io/cloud-provider-gcp/providers/gce"
	"k8s.io/klog/v2"
)

const nodeTopologyCRName = "default"

var (
	// nodeTopologyKeyFun maps node to a namespaced name as key for the task queue.
	nodeTopologyKeyFun = cache.DeletionHandlingMetaNamespaceKeyFunc
	// nodeTopologyReconcileFakeNode triggers periodic re-synchronization. Because
	// its fake node name won't match any real node, the syncer won't find it in the
	// nodeInformer cache, forcing a full reconciliation of the nodeTopology custom resource.
	nodeTopologyReconcileFakeNode = &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "/",
		},
	}
)

// NodeTopologySyncer processes nodetopology CR based on node add/update/delete events.
type NodeTopologySyncer struct {
	nodeTopologyClient nodetopologyclientset.Interface
	cloud              *gce.Cloud
	nodeLister         corelisters.NodeLister
}

func (syncer *NodeTopologySyncer) sync(key string) error {
	klog.InfoS("Syncing node topology CR for node", "key", key)
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		klog.ErrorS(err, "Failed to split namespace", "key", key)
		return nil
	}
	if syncer.nodeLister == nil {
		klog.ErrorS(err, "Nil syncer.nodeLister.")
		return nil
	}
	node, err := syncer.nodeLister.Get(name)
	if node == nil || err != nil {
		if errors.IsNotFound(err) {
			return nil // node no longer available, skip processing
		}
		klog.InfoS("Node not found or error, reconcile.", "node key", key, "error", err)
		err := syncer.reconcile()
		if err != nil {
			klog.ErrorS(err, "Failed to reconcile nodeTopology CR")
			return err
		}
	} else {
		err := syncer.updateNodeTopology(node)
		if err != nil {
			klog.ErrorS(err, "Failed to add or update nodeTopology CR")
			return err
		}

	}
	return nil
}

func (syncer *NodeTopologySyncer) reconcile() error {
	allNodes, err := syncer.nodeLister.List(labels.NewSelector())
	if err != nil {
		klog.ErrorS(err, "Failed to list all nodes from nodeInformer lister")
		return err
	}

	defaultSubnet, subnetPrefix, err := getSubnetWithPrefixFromURL(syncer.cloud.SubnetworkURL())
	if err != nil {
		klog.ErrorS(err, "Error parsing the default subnetworkURL")
		return err
	}
	updatedSubnetsMap := make(map[string]nodetopologyv1.SubnetConfig, 0)
	for _, node := range allNodes {
		hasSubnetLabel, nodeSubnet := getNodeSubnetLabel(node)
		if hasSubnetLabel {
			updatedSubnetsMap[nodeSubnet] = nodetopologyv1.SubnetConfig{
				Name:       nodeSubnet,
				SubnetPath: subnetPrefix + nodeSubnet,
			}
			klog.InfoS("Making node topology subnets list for all nodes with additional subnet", "subnet", nodeSubnet)
		}
	}
	updatedSubnetsMap[defaultSubnet] = nodetopologyv1.SubnetConfig{
		Name:       defaultSubnet,
		SubnetPath: subnetPrefix + defaultSubnet,
	}

	nodeTopologyCR, err := syncer.nodeTopologyClient.NetworkingV1().NodeTopologies().Get(context.TODO(), nodeTopologyCRName, metav1.GetOptions{})
	if err != nil {
		klog.ErrorS(err, "Failed to get NodeTopology", "nodeTopologyCR", nodeTopologyCRName)
		return err
	}
	updatedNodeTopologyCR := nodeTopologyCR.DeepCopy()
	updatedSubnets := make([]nodetopologyv1.SubnetConfig, 0)
	for _, s := range updatedSubnetsMap {
		updatedSubnets = append(updatedSubnets, s)
	}
	updatedNodeTopologyCR.Status.Subnets = updatedSubnets

	zoneSet := sets.NewString()
	for _, node := range allNodes {
		zone, err := getZoneFromNode(context.TODO(), syncer, node)
		if err != nil {
			return err
		}
		zoneSet.Insert(zone)
	}
	updatedNodeTopologyCR.Status.Zones = zoneSet.List()

	_, updateErr := syncer.nodeTopologyClient.NetworkingV1().NodeTopologies().UpdateStatus(context.TODO(), updatedNodeTopologyCR, metav1.UpdateOptions{})
	if updateErr != nil {
		klog.ErrorS(updateErr, "Error updating nodeTopology CR", "nodetopologyCR", nodeTopologyCRName)
		return updateErr
	}
	klog.InfoS("Successfully reconciled nodeTopolody CR")

	return nil
}

func (syncer *NodeTopologySyncer) updateNodeTopology(node *v1.Node) error {
	hasSubnetLabel, nodeSubnet := getNodeSubnetLabel(node)

	defaultSubnet, subnetPrefix, err := getSubnetWithPrefixFromURL(syncer.cloud.SubnetworkURL())
	if err != nil {
		klog.Errorf("Error parsing the default subnetworkURL, err: %v", err)
		return err
	}

	nodeTopologyCR, err := syncer.nodeTopologyClient.NetworkingV1().NodeTopologies().Get(context.TODO(), nodeTopologyCRName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("Failed to get NodeTopology: %v, err: %v", nodeTopologyCRName, err)
		return err
	}

	zone, zoneErr := getZoneFromNode(context.TODO(), syncer, node)

	var subnetsToAdd []nodetopologyv1.SubnetConfig
	if !isSubnetInCR(defaultSubnet, nodeTopologyCR) {
		subnetsToAdd = append(subnetsToAdd, nodetopologyv1.SubnetConfig{
			Name:       defaultSubnet,
			SubnetPath: subnetPrefix + defaultSubnet,
		})
	}

	if hasSubnetLabel && nodeSubnet != defaultSubnet && !isSubnetInCR(nodeSubnet, nodeTopologyCR) {
		subnetsToAdd = append(subnetsToAdd, nodetopologyv1.SubnetConfig{
			Name:       nodeSubnet,
			SubnetPath: subnetPrefix + nodeSubnet,
		})
	}

	var zonesToAdd []string
	if zoneErr == nil && !isZoneInCR(zone, nodeTopologyCR) {
		zonesToAdd = append(zonesToAdd, zone)
	}

	if len(subnetsToAdd) == 0 && len(zonesToAdd) == 0 {
		if zoneErr != nil {
			klog.V(4).InfoS("Waiting for zone info to be populated, forcing retry", "node", node.Name)
			return zoneErr
		}
		klog.V(2).InfoS("Both subnet and zone are already up to date, skipping", "node", node.Name)
		return nil
	}

	updatedCR := nodeTopologyCR.DeepCopy()
	if updatedCR.Status.Subnets == nil {
		updatedCR.Status.Subnets = []nodetopologyv1.SubnetConfig{}
	}
	updatedCR.Status.Subnets = append(updatedCR.Status.Subnets, subnetsToAdd...)

	// We always expect zones field in the status.
	if updatedCR.Status.Zones == nil {
		updatedCR.Status.Zones = []string{}
	}
	updatedCR.Status.Zones = append(updatedCR.Status.Zones, zonesToAdd...)

	_, updateErr := syncer.nodeTopologyClient.NetworkingV1().NodeTopologies().UpdateStatus(context.TODO(), updatedCR, metav1.UpdateOptions{})
	if updateErr != nil {
		klog.Errorf("Error updating the CR: %v, err: %v", nodeTopologyCRName, updateErr)
		return updateErr
	}

	for _, subnet := range subnetsToAdd {
		klog.V(2).Infof("Successfully add the subnet %v to the nodetopology CR", subnet.Name)
	}
	for _, z := range zonesToAdd {
		klog.V(2).Infof("Successfully add zone %v to the nodetopology CR", z)
	}

	if zoneErr != nil {
		klog.ErrorS(zoneErr, "Error updating zone for nodeTopology CR", "nodetopologyCR", nodeTopologyCRName, "node", node.Name)
		return zoneErr
	}

	return nil
}

// getNodeSubnetLabel returns true if the node has subnet label along with the subnet
func getNodeSubnetLabel(node *v1.Node) (bool, string) {
	subnet, foundSubnet := node.Labels[utilnode.NodePoolSubnetLabelPrefix]
	if !foundSubnet {
		return false, ""
	}
	return true, subnet
}

func getSubnetWithPrefixFromURL(url string) (subnetName string, subnetPrefix string, err error) {
	projectsPrefix := "projects/"

	// Get the path starting from "projects"
	startIndex := strings.Index(url, projectsPrefix)
	if startIndex == -1 {
		err = fmt.Errorf("'projects/' not found in the url string")
		return
	}
	projectsPath := url[startIndex:]

	parts := strings.Split(projectsPath, "/")
	if len(parts) < 2 {
		err = fmt.Errorf("Could not split the path into its parts")
		return
	}

	// Last part is the subnet name
	subnetName = parts[len(parts)-1]

	// Everything before the last part is the prefix
	subnetPrefixParts := parts[:len(parts)-1]
	subnetPrefix = strings.Join(subnetPrefixParts, "/") + "/"
	return
}

func isZoneInCR(nodeZone string, nodeTopologyCR *nodetopologyv1.NodeTopology) bool {
	for _, zone := range nodeTopologyCR.Status.Zones {
		if zone == nodeZone {
			return true
		}
	}
	return false
}

func isSubnetInCR(nodeSubnet string, nodeTopologyCR *nodetopologyv1.NodeTopology) bool {
	for _, subnet := range nodeTopologyCR.Status.Subnets {
		if subnet.Name == nodeSubnet {
			return true
		}
	}
	return false
}

func getZoneFromNode(ctx context.Context, syncer *NodeTopologySyncer, node *v1.Node) (string, error) {
	providerID := node.Spec.ProviderID
	if providerID == "" {
		err := fmt.Errorf("node doesn't have providerID")
		klog.ErrorS(err, "node doesn't have providerID", "node", node.Name)
		return "", err
	}

	nodeZoneConfig, err := syncer.cloud.GetZoneByProviderID(ctx, providerID)
	if err != nil {
		klog.ErrorS(err, "Failed to get zone information for node", "node", node.Name, "providerID", providerID)
		return "", err
	}

	return nodeZoneConfig.FailureDomain, nil
}
