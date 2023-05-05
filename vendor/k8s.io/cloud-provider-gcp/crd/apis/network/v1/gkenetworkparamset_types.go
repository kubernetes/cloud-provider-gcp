package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:storageversion
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GKENetworkParamSet represent GKE specific parameters for the network.
type GKENetworkParamSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GKENetworkParamSetSpec   `json:"spec,omitempty"`
	Status GKENetworkParamSetStatus `json:"status,omitempty"`
}

// DeviceModeType defines mode in which the devices will be used by the Pod
// +kubebuilder:validation:Enum=DPDK-VFIO;NetDevice
type DeviceModeType string

const (
	// DPDKVFIO indicates that NICs are bound to vfio-pci driver
	DPDKVFIO DeviceModeType = "DPDK-VFIO"
	// NetDevice indicates that NICs are bound to kernel driver and used as net device
	NetDevice DeviceModeType = "NetDevice"
)

// SecondaryRanges represents ranges of network addresses.
type SecondaryRanges struct {
	// +kubebuilder:validation:MinItems:=1
	RangeNames []string `json:"rangeNames"`
}

// GKENetworkParamSetSpec contains the specifications for network object
type GKENetworkParamSetSpec struct {
	// VPC speficies the VPC to which the network belongs.
	// +required
	VPC string `json:"vpc"`

	// VPCSubnet is the path of the VPC subnet
	// +required
	VPCSubnet string `json:"vpcSubnet"`

	// DeviceMode indicates the mode in which the devices will be used by the Pod.
	// This field is required and valid only for "Device" typed network
	// +optional
	DeviceMode DeviceModeType `json:"deviceMode"`

	// PodIPv4Ranges specify the names of the secondary ranges of the VPC subnet
	// used to allocate pod IPs for the network.
	// This field is required and valid only for L3 typed network
	// +optional
	PodIPv4Ranges *SecondaryRanges `json:"podIPv4Ranges,omitempty"`
}

// NetworkRanges represents ranges of network addresses.
type NetworkRanges struct {
	// +kubebuilder:validation:MinItems:=1
	CIDRBlocks []string `json:"cidrBlocks"`
}

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

// GKENetworkParamSetStatus contains the status information related to the network.
type GKENetworkParamSetStatus struct {
	// PodCIDRs specifies the CIDRs from which IPs will be used for Pod interfaces
	// +optional
	PodCIDRs *NetworkRanges `json:"podCIDRs,omitempty"`

	// Conditions is a field representing the current conditions of the GKENetworkParamSet.
	//
	// Known condition types are:
	//
	// * "Ready"
	//
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// NetworkName specifies which Network object is currently referencing this GKENetworkParamSet
	// +optional
	NetworkName string `json:"networkName"`
}

// +genclient
// +genclient:nonNamespaced
// +genclient:onlyVerbs=get
// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GKENetworkParamSetList contains a list of GKENetworkParamSet resources.
type GKENetworkParamSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	// Items is a slice of GKENetworkParamset resources.
	Items []GKENetworkParamSet `json:"items"`
}
