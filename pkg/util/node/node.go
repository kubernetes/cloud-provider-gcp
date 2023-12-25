/*
Copyright 2015 The Kubernetes Authors.

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

package node

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clientset "k8s.io/client-go/kubernetes"
	networkv1 "k8s.io/cloud-provider-gcp/crd/apis/network/v1"
	"k8s.io/klog/v2"
)

// Labels definitions.
const (
	// NodePoolPodRangeLabelPrefix is the prefix for the default Pod range
	// name for the node and it can be different with cluster Pod range
	NodePoolPodRangeLabelPrefix = "cloud.google.com/gke-np-default-pod-range"
	// NodePoolSubnetLabelPrefix is the prefix for the default subnet
	// name for the node
	NodePoolSubnetLabelPrefix = "cloud.google.com/gke-np-default-subnet"
)

type nodeForConditionPatch struct {
	Status nodeStatusForPatch `json:"status"`
}

type nodeStatusForPatch struct {
	Conditions []v1.NodeCondition `json:"conditions"`
}

// SetNodeCondition updates specific node condition with patch operation.
func SetNodeCondition(c clientset.Interface, node types.NodeName, condition v1.NodeCondition) error {
	generatePatch := func(condition v1.NodeCondition) ([]byte, error) {
		patch := nodeForConditionPatch{
			Status: nodeStatusForPatch{
				Conditions: []v1.NodeCondition{
					condition,
				},
			},
		}
		patchBytes, err := json.Marshal(&patch)
		if err != nil {
			return nil, err
		}
		return patchBytes, nil
	}
	condition.LastHeartbeatTime = metav1.NewTime(time.Now())
	patch, err := generatePatch(condition)
	if err != nil {
		return nil
	}
	_, err = c.CoreV1().Nodes().PatchStatus(context.TODO(), string(node), patch)
	return err
}

type nodeForCIDRMergePatch struct {
	Spec nodeSpecForMergePatch `json:"spec"`
}

type nodeSpecForMergePatch struct {
	PodCIDR  string   `json:"podCIDR"`
	PodCIDRs []string `json:"podCIDRs,omitempty"`
}

// PatchNodeCIDRs patches the specified node.CIDR=cidrs[0] and node.CIDRs to the given value.
func PatchNodeCIDRs(c clientset.Interface, node types.NodeName, cidrs []string) error {
	// set the pod cidrs list and set the old pod cidr field
	patch := nodeForCIDRMergePatch{
		Spec: nodeSpecForMergePatch{
			PodCIDR:  cidrs[0],
			PodCIDRs: cidrs,
		},
	}

	patchBytes, err := json.Marshal(&patch)
	if err != nil {
		return fmt.Errorf("failed to json.Marshal CIDR: %v", err)
	}
	klog.V(4).Infof("cidrs patch bytes are:%s", string(patchBytes))
	if _, err := c.CoreV1().Nodes().Patch(context.TODO(), string(node), types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{}); err != nil {
		return fmt.Errorf("failed to patch node CIDR: %v", err)
	}
	return nil
}

// PatchNodeMultiNetwork patches the Node's annotations and capacity for MN.
func PatchNodeMultiNetwork(c clientset.Interface, node *v1.Node) error {
	annotation := make(map[string]string)
	if val, ok := node.Annotations[networkv1.NorthInterfacesAnnotationKey]; ok {
		annotation[networkv1.NorthInterfacesAnnotationKey] = val
	}
	if val, ok := node.Annotations[networkv1.MultiNetworkAnnotationKey]; ok {
		annotation[networkv1.MultiNetworkAnnotationKey] = val
	}

	if len(annotation) > 0 {
		raw, err := json.Marshal(annotation)
		if err != nil {
			return fmt.Errorf("failed to build patch bytes for multi-networking: %w", err)
		}
		if _, err := c.CoreV1().Nodes().Patch(context.TODO(), node.Name, types.StrategicMergePatchType, []byte(fmt.Sprintf(`{"metadata":{"annotations":%s}}`, raw)), metav1.PatchOptions{}); err != nil {
			return fmt.Errorf("unable to apply patch for multi-network annotation: %v", err)
		}
	}
	// Prepare patch bytes for the node update.
	patchBytes, err := json.Marshal([]interface{}{
		map[string]interface{}{
			"op":    "add",
			"path":  "/status/capacity",
			"value": node.Status.Capacity,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to build patch bytes for multi-networking: %w", err)
	}
	// Since dynamic network addition/deletion is a use case to be supported, we aspire to build these annotations and IP capacities every time from scratch.
	// Hence, we are using a JSON patch merge strategy instead of strategic merge strategy on the node during update.
	if _, err = c.CoreV1().Nodes().Patch(context.TODO(), node.Name, types.JSONPatchType, patchBytes, metav1.PatchOptions{}, "status"); err != nil {
		return fmt.Errorf("failed to patch node for multi-networking: %w", err)
	}
	return nil
}
