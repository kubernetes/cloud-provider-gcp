package v1

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ###############
// ### Network ###

// NetworkConditionType is the type for status conditions on
// a Network. This type should be used with the
// NetworkStatus.Conditions field.
type NetworkConditionType string

const (
	// NetworkConditionStatusReady is the condition type that holds
	// if the Network object is validated
	NetworkConditionStatusReady NetworkConditionType = "Ready"

	// NetworkConditionStatusParamsReady is the condition type that holds
	// if the params object referenced by Network is validated
	NetworkConditionStatusParamsReady NetworkConditionType = "ParamsReady"
)

// NetworkReadyConditionReason defines the set of reasons that explain why a
// particular Network Ready condition type has been raised.
type NetworkReadyConditionReason string

const (
	// ParamsNotReady indicates that the resource referenced in params is not ready.
	ParamsNotReady NetworkReadyConditionReason = "ParamsNotReady"
)

// ##########################
// ### GKENetworkParamSet ###

// GKENetworkParamSetConditionType is the type for status conditions on
// a GKENetworkParamSet. This type should be used with the
// GKENetworkParamSetStatus.Conditions field.
type GKENetworkParamSetConditionType string

const (
	// GKENetworkParamSetStatusReady is the condition type that holds
	// if the GKENetworkParamSet object is validated
	GKENetworkParamSetStatusReady GKENetworkParamSetConditionType = "Ready"
)

// GKENetworkParamSetConditionReason defines the set of reasons that explain why a
// particular GKENetworkParamSet condition type has been raised.
type GKENetworkParamSetConditionReason string

const (
	// SubnetNotFound indicates that the specified subnet was not found.
	SubnetNotFound GKENetworkParamSetConditionReason = "SubnetNotFound"
	// SecondaryRangeNotFound indicates that the specified secondary range was not found.
	SecondaryRangeNotFound GKENetworkParamSetConditionReason = "SecondaryRangeNotFound"
	// DeviceModeCantBeUsedWithSecondaryRange indicates that device mode was used with a secondary range.
	DeviceModeCantBeUsedWithSecondaryRange GKENetworkParamSetConditionReason = "DeviceModeCantBeUsedWithSecondaryRange"
	// DeviceModeVPCAlreadyInUse indicates that the VPC is already in use by another GKENetworkParamSet resource.
	DeviceModeVPCAlreadyInUse GKENetworkParamSetConditionReason = "DeviceModeVPCAlreadyInUse"
	// DeviceModeCantUseDefaultVPC indicates that a device mode GKENetworkParamSet cannot use the default VPC.
	DeviceModeCantUseDefaultVPC GKENetworkParamSetConditionReason = "DeviceModeCantUseDefaultVPC"
	// DPDKUnsupported indicates that DPDK device mode is not supported on the current cluster.
	DPDKUnsupported GKENetworkParamSetConditionReason = "DPDKUnsupported"
)

// GNPNetworkParamsReadyConditionReason defines the set of reasons that explains
// the ParamsReady condition on the referencing Network resource.
type GNPNetworkParamsReadyConditionReason string

const (
	// L3SecondaryMissing indicates that the L3 type Network resource is
	// referencing a GKENetworkParamSet with secondary range unspecified.
	L3SecondaryMissing GNPNetworkParamsReadyConditionReason = "L3SecondaryMissing"
	// L3DeviceModeExists indicates that the L3 type Network resource is
	// referencing a GKENetworkParamSet with device mode specified.
	L3DeviceModeExists GNPNetworkParamsReadyConditionReason = "L3DeviceModeExists"
	// DeviceModeMissing indicates that the Device type Network resource is
	// referencing a GKENetworkParamSet with device mode unspecified.
	DeviceModeMissing GNPNetworkParamsReadyConditionReason = "DeviceModeMissing"
	// DeviceSecondaryExists indicates that the Device type Network resource is
	// referencing a GKENetworkParamSet with a secondary range specified.
	DeviceSecondaryExists GNPNetworkParamsReadyConditionReason = "DeviceSecondaryExists"
)

// ###############
// ### helpers ###

// GetCondition returns the "condType" condition from the obj. Only Network and
// GKENetworkParamSet types are supported. Returns nil when not found.
func GetCondition(obj interface{}, condType string) (*metav1.Condition, error) {
	var conditions []metav1.Condition

	switch obj.(type) {
	case *Network:
		conditions = obj.(*Network).Status.Conditions
	case *GKENetworkParamSet:
		conditions = obj.(*GKENetworkParamSet).Status.Conditions
	default:
		return nil, fmt.Errorf("unsupported type: %T", obj)
	}

	for _, cond := range conditions {
		if cond.Type == condType {
			return cond.DeepCopy(), nil
		}
	}
	return nil, nil
}

// IsReady returns true when the specified obj Ready condition is True.
// Only Network and GKENetworkParamSet types are supported.
func IsReady(obj interface{}) (bool, error) {
	cond, err := GetCondition(obj, "Ready")
	if err != nil {
		return false, err
	}
	if cond != nil {
		return cond.Status == metav1.ConditionTrue, nil
	}
	return false, nil
}
