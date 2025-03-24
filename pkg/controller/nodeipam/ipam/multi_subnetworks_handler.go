package ipam

import (
	"context"
	"fmt"
	nodetopologyv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/nodetopology/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilnode "k8s.io/cloud-provider-gcp/pkg/util/node"
	"k8s.io/klog/v2"
	"strings"
)

const nodeTopologyCRName = "default"

func (ca *cloudCIDRAllocator) updateNodeTopology(node *v1.Node) error {
	ctx := context.Background()

	hasSubnetLabel, nodeSubnet := getNodeSubnetLabel(node)
	if !hasSubnetLabel {
		klog.V(2).Infof("Cannot find the subnet label on the node: %v", node.Name)
	}

	defaultSubnet, subnetPrefix, err := getSubnetWithPrefixFromURL(ca.cloud.SubnetworkURL())
	if err != nil {
		klog.Errorf("Error parsing the default subnetworkURL, err: %v", err)
		return err
	}

	nodeTopologyCR, err := ca.nodeTopologyClient.NetworkingV1().NodeTopologies().Get(ctx, nodeTopologyCRName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("Failed to get NodeTopology: %v, err: %v", nodeTopologyCRName, err)
		return err
	}

	crSubnets := nodeTopologyCR.Status.Subnets
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
		_, updateErr := ca.nodeTopologyClient.NetworkingV1().NodeTopologies().UpdateStatus(ctx, updatedCR, metav1.UpdateOptions{})
		if updateErr != nil {
			klog.Errorf("Error updating the CR: %v, err: %v", nodeTopologyCRName, updateErr)
			return updateErr
		} else {
			klog.V(2).Infof("Successfully added the default subnet %v to nodetopology CR", defaultSubnet)
		}

		return nil
	}

	if !hasSubnetLabel {
		klog.V(2).Infof("Default subnetwork is already updated in the CR.")
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

	_, updateErr := ca.nodeTopologyClient.NetworkingV1().NodeTopologies().UpdateStatus(ctx, updatedCR, metav1.UpdateOptions{})
	if updateErr != nil {
		klog.Errorf("Error updating the CR: %v, err: %v", nodeTopologyCRName, updateErr)
		return updateErr
	} else {
		klog.V(2).Infof("Successfully add the subnet %v to the nodetopology CR", nodeSubnet)
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

	// 1. Get the path starting from "projects"
	startIndex := strings.Index(url, projectsPrefix)
	if startIndex == -1 {
		err = fmt.Errorf("'projects/' not found in the url string")
		return
	}
	projectsPath := url[startIndex:]

	// Split the path into two parts
	parts := strings.Split(projectsPath, "/")
	if len(parts) < 2 {
		err = fmt.Errorf("Could not split the path into resource type and name")
		return
	}

	// Last part is the subnet name
	subnetName = parts[len(parts)-1]

	// Everything before the last part is the prefix
	subnetPrefixParts := parts[:len(parts)-1]
	subnetPrefix = strings.Join(subnetPrefixParts, "/") + "/"
	return
}
