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

	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud"
	"google.golang.org/api/compute/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	networkv1 "k8s.io/cloud-provider-gcp/crd/apis/network/v1"
)

type gnpValidation struct {
	IsValid      bool
	ErrorReason  networkv1.GKENetworkParamSetConditionReason
	ErrorMessage string
}

func (val *gnpValidation) toCondition() v1.Condition {
	condition := v1.Condition{}

	if val.IsValid {
		condition.Status = v1.ConditionTrue
		condition.Reason = string(networkv1.GNPReady)
	} else {
		condition.Status = v1.ConditionFalse
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

	network, err := c.gceCloud.GetNetwork(params.Spec.VPC)
	if err != nil || network == nil {
		return &gnpValidation{
			IsValid:      false,
			ErrorReason:  networkv1.VPCNotFound,
			ErrorMessage: fmt.Sprintf("VPC: %s not found", params.Spec.VPC),
		}, nil
	}

	//check if both deviceMode and secondary ranges are unspecified
	isSecondaryRangeSpecified := params.Spec.PodIPv4Ranges != nil && len(params.Spec.PodIPv4Ranges.RangeNames) > 0
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
		gnpList, err := c.networkClientset.NetworkingV1().GKENetworkParamSets().List(ctx, v1.ListOptions{})
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

func (val *gnpNetworkCrossValidation) toCondition() v1.Condition {
	condition := v1.Condition{}

	if val.IsValid {
		condition.Status = v1.ConditionTrue
		condition.Reason = string(networkv1.GNPParamsReady)
	} else {
		condition.Status = v1.ConditionFalse
		condition.Reason = string(val.ErrorReason)
		condition.Message = val.ErrorMessage
	}

	condition.Type = string(networkv1.NetworkConditionStatusParamsReady)

	return condition
}

// crossValidateNetworkAndGnp validates a given network and GNP object are compatible
func crossValidateNetworkAndGnp(network *networkv1.Network, params *networkv1.GKENetworkParamSet) *gnpNetworkCrossValidation {
	isSecondaryRangeSpecified := params.Spec.PodIPv4Ranges != nil && len(params.Spec.PodIPv4Ranges.RangeNames) > 0

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
