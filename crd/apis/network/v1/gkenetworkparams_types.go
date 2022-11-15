package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:storageversion
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GKENetworkParams represent GKE specific parameters for the network.
type GKENetworkParams struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GKENetworkParamsSpec   `json:"spec,omitempty"`
	Status GKENetworkParamsStatus `json:"status,omitempty"`
}

// DeviceModeType defines mode in which the devices will be used by the Pod
// +kubebuilder:validation:Enum=DPDK-UIO;DPDK-VFIO;NetDevice
type DeviceModeType string

const (
	// DPDKUIO indicates that NICs are bound to uio_pci_generic driver
	DPDKUIO DeviceModeType = "DPDK-UIO"
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

// GKENetworkParamsSpec contains the specifications for network object
type GKENetworkParamsSpec struct {
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

// GKENetworkParamsStatus contains the status information related to the network.
type GKENetworkParamsStatus struct {
	// PodCIDRs specifies the CIDRs from which IPs will be used for Pod interfaces
	// +optional
	PodCIDRs *NetworkRanges `json:"podCIDRs,omitempty"`
}
