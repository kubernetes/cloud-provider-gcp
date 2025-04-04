package ipam

import (
	"context"
	"fmt"
	"strings"

	corelisters "k8s.io/client-go/listers/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	nodetopologyv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/nodetopology/v1"
	nodetopologyclientset "github.com/GoogleCloudPlatform/gke-networking-api/client/nodetopology/clientset/versioned"
	utilnode "k8s.io/cloud-provider-gcp/pkg/util/node"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/cloud-provider-gcp/providers/gce"
	"k8s.io/klog/v2"
)

const nodeTopologyCRName = "default"

var (
	// nodeTopologyReconciliationKey is a queue entry for nodeTopology reconciliation (used for node deletion).
	nodeTopologyReconciliationKey = &v1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "nodeTopologyReconciliationKey"},
	}
)

type NodeTopologySyncer struct {
	nodeTopologyClient 	nodetopologyclientset.Interface
	cloud              	*gce.Cloud
	nodeLister 			corelisters.NodeLister
	ctx                	context.Context
}

func (syncer *NodeTopologySyncer) sync(node *v1.Node) error {
	klog.V(0).Infof("syncing node topology CR for node item %v", node.GetName())
	if node == nodeTopologyReconciliationKey {
		err := syncer.reconcile()
		if err != nil {
			klog.Errorf("Failed to reconcile nodeTopology CR")
			return err
		}
	} else {
		err := syncer.updateNodeTopology(node)
		if err != nil {
			klog.Errorf("Failed to add or update nodeTopology CR")
			return err
		}
		
	}
	return nil
}

func (syncer *NodeTopologySyncer) reconcile() error {
	klog.V(0).Infof("Reconciling node topology CR.")
	if syncer.nodeLister == nil {
		return nil
	}
	allNodes, err := syncer.nodeLister.List(labels.NewSelector())
	if err != nil {
		klog.Errorf("Failed to list all nodes from nodeInformer lister.")
		return err
	}

	defaultSubnet, subnetPrefix, err := getSubnetWithPrefixFromURL(syncer.cloud.SubnetworkURL())
	if err != nil {
		klog.Errorf("Error parsing the default subnetworkURL, err: %v", err)
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
			klog.V(0).Infof("Making node topology subnets list, adding subnet %v.", nodeSubnet)
		}
	}
	updatedSubnetsMap[defaultSubnet] = nodetopologyv1.SubnetConfig{
		Name:       defaultSubnet,
		SubnetPath: subnetPrefix + defaultSubnet,
	}

	nodeTopologyCR, err := syncer.nodeTopologyClient.NetworkingV1().NodeTopologies().Get(syncer.ctx, nodeTopologyCRName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("Failed to get NodeTopology: %v, err: %v", nodeTopologyCRName, err)
		return err
	}
	updatedNodeTopologyCR := nodeTopologyCR.DeepCopy()
	updatedSubnets := make([]nodetopologyv1.SubnetConfig, 0)
	for _, s := range updatedSubnetsMap {
		updatedSubnets = append(updatedSubnets, s)
	}
	updatedNodeTopologyCR.Status.Subnets = updatedSubnets
	_, updateErr := syncer.nodeTopologyClient.NetworkingV1().NodeTopologies().UpdateStatus(syncer.ctx, updatedNodeTopologyCR, metav1.UpdateOptions{})
	if updateErr != nil {
		klog.Errorf("Error updating nodeTopology CR: %v, err: %v", nodeTopologyCRName, updateErr)
		return updateErr
	}
	klog.V(2).Infof("Successfully reconciled nodeTopolody CR.")
	return nil
}

func (syncer *NodeTopologySyncer) updateNodeTopology(node *v1.Node) error {
	hasSubnetLabel, nodeSubnet := getNodeSubnetLabel(node)

	defaultSubnet, subnetPrefix, err := getSubnetWithPrefixFromURL(syncer.cloud.SubnetworkURL())
	if err != nil {
		klog.Errorf("Error parsing the default subnetworkURL, err: %v", err)
		return err
	}

	nodeTopologyCR, err := syncer.nodeTopologyClient.NetworkingV1().NodeTopologies().Get(syncer.ctx, nodeTopologyCRName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("Failed to get NodeTopology: %v, err: %v", nodeTopologyCRName, err)
		return err
	}

	crSubnets := nodeTopologyCR.Status.Subnets
	// We will always add the default subnet to the CR.
	// We do not let additional subnetworks to be added during cluster creation.
	// Hence, we are sure to always add the default subnet first.
	// The reconciliation logic will also ensure this behavior.
	if crSubnets == nil {
		// Add the default subnet to the node topology CR
		klog.V(2).Infof("No subnets found in the cr, adding the default subnet")
		updatedCR := nodeTopologyCR.DeepCopy()
		updatedCR.Status.Subnets = append(updatedCR.Status.Subnets, nodetopologyv1.SubnetConfig{
			Name:       defaultSubnet,
			SubnetPath: subnetPrefix + defaultSubnet,
		})
		// We always expect zones field in the status.
		if updatedCR.Status.Zones == nil {
			updatedCR.Status.Zones = []string{}
		}
		_, updateErr := syncer.nodeTopologyClient.NetworkingV1().NodeTopologies().UpdateStatus(syncer.ctx, updatedCR, metav1.UpdateOptions{})
		if updateErr != nil {
			klog.Errorf("Error updating the CR: %v, err: %v", nodeTopologyCRName, updateErr)
			return updateErr
		}

		klog.V(2).Infof("Successfully added the default subnet %v to nodetopology CR", defaultSubnet)

		return nil
	}

	if !hasSubnetLabel {
		klog.V(2).Infof("No additional subnet detected. Default subnetwork is added to the CR.")
		return nil
	}

	// Check if subnet already exists in the CR
	for _, subnet := range crSubnets {
		if subnet.Name == nodeSubnet {
			klog.V(2).Infof("The subnet %s already exists in the node topology CR", nodeSubnet)
			return nil
		}
	}

	// We have a new subnet that should be added to the CR
	// We assume all the subnets are in the same project and region
	updatedCR := nodeTopologyCR.DeepCopy()
	updatedCR.Status.Subnets = append(updatedCR.Status.Subnets, nodetopologyv1.SubnetConfig{
		Name:       nodeSubnet,
		SubnetPath: subnetPrefix + nodeSubnet,
	})
	// We always expect zones field in the status.
	if updatedCR.Status.Zones == nil {
		updatedCR.Status.Zones = []string{}
	}

	_, updateErr := syncer.nodeTopologyClient.NetworkingV1().NodeTopologies().UpdateStatus(syncer.ctx, updatedCR, metav1.UpdateOptions{})
	if updateErr != nil {
		klog.Errorf("Error updating the CR: %v, err: %v", nodeTopologyCRName, updateErr)
		return updateErr
	}

	klog.V(2).Infof("Successfully add the subnet %v to the nodetopology CR", nodeSubnet)

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
