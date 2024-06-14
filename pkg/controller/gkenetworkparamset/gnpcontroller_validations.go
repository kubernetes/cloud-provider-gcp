/*
Copyright 2023 The Kubernetes Authors.

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

package gkenetworkparamset

import (
	"context"
	"fmt"

	networkv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/network/v1"
	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud"
	"google.golang.org/api/compute/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilnode "k8s.io/cloud-provider-gcp/pkg/util/node"
	"k8s.io/klog/v2"
	"k8s.io/utils/strings/slices"
)

type gnpValidation struct {
	IsValid      bool
	ErrorReason  networkv1.GKENetworkParamSetConditionReason
	ErrorMessage string
}

func (val *gnpValidation) toCondition() metav1.Condition {
	condition := metav1.Condition{}

	if val.IsValid {
		condition.Status = metav1.ConditionTrue
		condition.Reason = string(networkv1.GNPReady)
	} else {
		condition.Status = metav1.ConditionFalse
		condition.Reason = string(val.ErrorReason)
		condition.Message = val.ErrorMessage
	}

	condition.Type = string(networkv1.GKENetworkParamSetStatusReady)

	return condition
}

// getAndValidateSubnet validates that the subnet is present in params and exists in GCP.
func (c *Controller) getAndValidateSubnet(ctx context.Context, params *networkv1.GKENetworkParamSet) (*compute.Subnetwork, *gnpValidation) {
	if params.Spec.VPCSubnet == "" {
		return nil, &gnpValidation{
			IsValid:      false,
			ErrorReason:  networkv1.SubnetNotFound,
			ErrorMessage: "subnet not specified",
		}
	}

	// Check if Subnet exists
	subnet, err := c.gceCloud.GetSubnetwork(c.gceCloud.Region(), params.Spec.VPCSubnet)
	if err != nil || subnet == nil {
		return nil, &gnpValidation{
			IsValid:      false,
			ErrorReason:  networkv1.SubnetNotFound,
			ErrorMessage: fmt.Sprintf("subnet: %s not found in VPC: %s", params.Spec.VPCSubnet, params.Spec.VPC),
		}
	}

	return subnet, &gnpValidation{IsValid: true}
}

func (c *Controller) validateGKENetworkParamSet(ctx context.Context, params *networkv1.GKENetworkParamSet, subnet *compute.Subnetwork) (*gnpValidation, error) {

	//check if vpc exists
	if params.Spec.VPC == "" {
		return &gnpValidation{
			IsValid:      false,
			ErrorReason:  networkv1.VPCNotFound,
			ErrorMessage: "VPC not specified",
		}, nil
	}

	if !c.gceCloud.OnXPN() {
		network, err := c.gceCloud.GetNetwork(params.Spec.VPC)
		if err != nil || network == nil {
			return &gnpValidation{
				IsValid:      false,
				ErrorReason:  networkv1.VPCNotFound,
				ErrorMessage: fmt.Sprintf("VPC: %s not found", params.Spec.VPC),
			}, nil
		}
	}

	// check if both deviceMode and secondary ranges are unspecified
	isSecondaryRangeSpecified := hasRangeNames(params)
	isDeviceModeSpecified := params.Spec.DeviceMode != ""
	if !isSecondaryRangeSpecified && !isDeviceModeSpecified {
		return &gnpValidation{
			IsValid:      false,
			ErrorReason:  networkv1.SecondaryRangeAndDeviceModeUnspecified,
			ErrorMessage: "SecondaryRange and DeviceMode are unspecified. One must be specified.",
		}, nil
	}

	// Check if secondary range exists
	if isSecondaryRangeSpecified && !isDeviceModeSpecified {
		for _, rangeName := range params.Spec.PodIPv4Ranges.RangeNames {
			found := false
			for _, sr := range subnet.SecondaryIpRanges {
				if sr.RangeName == rangeName {
					found = true
					break
				}
			}
			if !found {
				return &gnpValidation{
					IsValid:      false,
					ErrorReason:  networkv1.SecondaryRangeNotFound,
					ErrorMessage: fmt.Sprintf("secondary range: %s not found in subnet: %s", rangeName, params.Spec.VPCSubnet),
				}, nil
			}
		}
	}

	// Check if deviceMode is specified at the same time as secondary range
	if isSecondaryRangeSpecified && isDeviceModeSpecified {
		return &gnpValidation{
			IsValid:      false,
			ErrorReason:  networkv1.DeviceModeCantBeUsedWithSecondaryRange,
			ErrorMessage: "deviceMode and secondary range can not be specified at the same time",
		}, nil
	}

	//if GNP with deviceMode and The referencing VPC is the default VPC
	if isDeviceModeSpecified {
		networkResource, err := cloud.ParseResourceURL(c.gceCloud.NetworkURL())
		if err != nil {
			return nil, err
		}
		if params.Spec.VPC == networkResource.Key.Name {
			return &gnpValidation{
				IsValid:      false,
				ErrorReason:  networkv1.DeviceModeCantUseDefaultVPC,
				ErrorMessage: "GNP with deviceMode can't reference the default VPC",
			}, nil
		}
	}

	//if GNP with deviceMode and referencing VPC or Subnet is referenced in any other existing GNP
	if isDeviceModeSpecified {
		gnpList, err := c.networkClientset.NetworkingV1().GKENetworkParamSets().List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, err
		}
		for _, otherGNP := range gnpList.Items {
			isDifferentGNP := params.Name != otherGNP.Name
			isMatchingVPC := params.Spec.VPC == otherGNP.Spec.VPC
			isMatchingSubnet := params.Spec.VPCSubnet == otherGNP.Spec.VPCSubnet
			isParamsNewer := params.CreationTimestamp.After(otherGNP.CreationTimestamp.Time)

			if isDifferentGNP && isMatchingVPC && isParamsNewer {
				return &gnpValidation{
					IsValid:      false,
					ErrorReason:  networkv1.DeviceModeVPCAlreadyInUse,
					ErrorMessage: fmt.Sprintf("GNP with deviceMode can't reference a VPC already in use. VPC '%s' is already in use by '%s'", otherGNP.Spec.VPC, otherGNP.Name),
				}, nil
			}

			if isDifferentGNP && isMatchingSubnet && isParamsNewer {
				return &gnpValidation{
					IsValid:      false,
					ErrorReason:  networkv1.DeviceModeSubnetAlreadyInUse,
					ErrorMessage: fmt.Sprintf("GNP with deviceMode can't reference a subnet already in use. Subnet '%s' is already in use by '%s'", otherGNP.Spec.VPC, otherGNP.Name),
				}, nil
			}
		}
	}

	return &gnpValidation{IsValid: true}, nil
}

type gnpNetworkCrossValidation struct {
	IsValid      bool
	ErrorReason  networkv1.GNPNetworkParamsReadyConditionReason
	ErrorMessage string
}

func (val *gnpNetworkCrossValidation) toCondition() metav1.Condition {
	condition := metav1.Condition{}

	if val.IsValid {
		condition.Status = metav1.ConditionTrue
		condition.Reason = string(networkv1.GNPParamsReady)
	} else {
		condition.Status = metav1.ConditionFalse
		condition.Reason = string(val.ErrorReason)
		condition.Message = val.ErrorMessage
	}

	condition.Type = string(networkv1.NetworkConditionStatusParamsReady)

	return condition
}

// crossValidateNetworkAndGnp validates a given network and GNP object are compatible
func crossValidateNetworkAndGnp(network *networkv1.Network, params *networkv1.GKENetworkParamSet) *gnpNetworkCrossValidation {
	isSecondaryRangeSpecified := hasRangeNames(params)

	if network.Spec.Type == networkv1.L3NetworkType {
		if !isSecondaryRangeSpecified {
			return &gnpNetworkCrossValidation{
				IsValid:      false,
				ErrorReason:  networkv1.L3SecondaryMissing,
				ErrorMessage: "L3 type network requires secondary range to be specified in params",
			}
		}
	}

	if network.Spec.Type == networkv1.DeviceNetworkType {
		if params.Spec.DeviceMode == "" {
			return &gnpNetworkCrossValidation{
				IsValid:      false,
				ErrorReason:  networkv1.DeviceModeMissing,
				ErrorMessage: "Device type network requires device mode to be specified in params",
			}
		}
	}

	return &gnpNetworkCrossValidation{
		IsValid: true,
	}
}

// nonDefaultParamsPodRanges returns true if the node has new Pod range that's not in the "default" params
func (c *Controller) nonDefaultParamsPodRanges(node *v1.Node) bool {
	defaultPodRanges, err := c.getParamsPodRanges(networkv1.DefaultPodNetworkName)
	if err != nil {
		klog.V(4).Infof("check new Pod range on node %q error: %v", node.Name, err)
		return false
	}
	v, ok := node.Labels[utilnode.NodePoolPodRangeLabelPrefix]
	// node pools can not create with overlapped pod ranges so that we can use `slices.Contains`
	if ok && v != "" && !slices.Contains(defaultPodRanges, v) {
		return true
	}
	return false
}

// getParamsPodRanges returns a list of Pod range names of the paramset and error
func (c *Controller) getParamsPodRanges(paramsName string) ([]string, error) {
	params, err := c.gkeNetworkParamsInformer.Lister().Get(paramsName)
	if err != nil {
		return nil, err
	}
	if hasRangeNames(params) {
		return params.Spec.PodIPv4Ranges.RangeNames, nil
	}
	return nil, fmt.Errorf("params %v does not have PodIPv4Ranges", params.Name)
}

// hasRangeNames returns true if RangeNames is specified, return false
// if PodIPv4Ranges is nil or length of RangeNames is 0
func hasRangeNames(params *networkv1.GKENetworkParamSet) bool {
	if params.Spec.PodIPv4Ranges != nil {
		if len(params.Spec.PodIPv4Ranges.RangeNames) > 0 {
			return true
		}
	}
	return false
}

// samePodIPv4Ranges returns true if both PodIPv4Rangess are nil or have the same RangeNames,
// returns false if either one is nil or has differnent element in the RangeNames list
func samePodIPv4Ranges(params *networkv1.GKENetworkParamSet, originalParams *networkv1.GKENetworkParamSet) bool {
	if !hasRangeNames(params) && !hasRangeNames(originalParams) {
		return true
	}
	if hasRangeNames(params) && hasRangeNames(originalParams) {
		return sameStringSlice(params.Spec.PodIPv4Ranges.RangeNames, originalParams.Spec.PodIPv4Ranges.RangeNames)
	}
	return false
}

// sameStringSlice returns true if two slices have the same elements
// regardless of the order
func sameStringSlice(x, y []string) bool {
	if len(x) != len(y) {
		return false
	}
	diff := make(map[string]int, len(x))
	for _, a := range x {
		diff[a]++
	}
	for _, b := range y {
		if _, ok := diff[b]; !ok {
			return false
		}
		diff[b]--
		if diff[b] == 0 {
			delete(diff, b)
		}
	}
	return len(diff) == 0
}
