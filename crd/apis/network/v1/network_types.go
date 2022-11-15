package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

const (
	// DefaultNetworkName is the network used by the VETH interface.
	DefaultNetworkName = "pod-network"
	// NetworkResourceKeyPrefix is the prefix for extended resource
	// name corresponding to the network.
	// e.g. "networking.gke.io.networks/my-network.IP"
	NetworkResourceKeyPrefix = "networking.gke.io.networks/"
)

// NetworkType is the type of network.
// +kubebuilder:validation:Enum=L2;L3
type NetworkType string

const (
	// L2NetworkType enables L2 connectivity on the network.
	L2NetworkType NetworkType = "L2"
	// L3NetworkType enables L3 connectivity on the network.
	L3NetworkType NetworkType = "L3"
)

// LifecycleType defines who manages the lifecycle of the network.
// +kubebuilder:validation:Enum=AnthosManaged;UserManaged
type LifecycleType string

const (
	// AnthosManagedLifecycle indicates that the Anthos will manage the Network
	// lifecycle.
	AnthosManagedLifecycle LifecycleType = "AnthosManaged"
	// UserManaged indicates that the user will manage the Network
	// Lifeycle and Anthos will not create or delete the network.
	UserManagedLifecycle LifecycleType = "UserManaged"
)

// ProviderType defines provider of the network.
// +kubebuilder:validation:Enum=GKE
type ProviderType string

const (
	// GKE indicates network provider is "GKE"
	GKE ProviderType = "GKE"
)

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:storageversion
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Network represent a logical network on the K8s Cluster.
// This logical network depends on the host networking setup on cluster nodes.
type Network struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NetworkSpec   `json:"spec,omitempty"`
	Status NetworkStatus `json:"status,omitempty"`
}

// NetworkSpec contains the specifications for network object
type NetworkSpec struct {
	// Type defines type of network.
	// Valid options include: L2, L3.
	// L2 network type enables L2 connectivity on the network.
	// L3 network type enables L3 connectivity on the network.
	// +required
	Type NetworkType `json:"type"`

	// Provider specifies the provider implementing this network, e.g. "GKE".
	Provider *ProviderType `json:"provider,omitempty"`

	// NodeInterfaceMatcher defines the matcher to discover the corresponding node interface associated with the network.
	// This field is required for L2 network.
	// +optional
	NodeInterfaceMatcher NodeInterfaceMatcher `json:"nodeInterfaceMatcher,omitempty"`

	// L2NetworkConfig includes all the network config related to L2 type network
	// +optional
	L2NetworkConfig *L2NetworkConfig `json:"l2NetworkConfig,omitempty"`

	// NetworkLifecycle specifies who manages the lifecycle of the network.
	// This field can only be used when L2NetworkConfig.VlanID is specified. Otherwise the value will be ignored. If
	// L2NetworkConfig.VlanID is specified and this field is empty, the value is assumed to be AnthosManaged.
	// +optional
	NetworkLifecycle *LifecycleType `json:"networkLifecycle,omitempty"`

	// Routes contains a list of routes for the network.
	// +optional
	Routes []Route `json:"routes,omitempty"`

	// Gateway4 defines the gateway IPv4 address for the network.
	// Required if ExternalDHCP4 is false or not set on L2 type network.
	// +optional
	Gateway4 *string `json:"gateway4,omitempty"`

	// Specifies the DNS configuration of the network.
	// Required if ExternalDHCP4 is false or not set on L2 type network.
	// +optional
	DNSConfig *DNSConfig `json:"dnsConfig,omitempty"`

	// ExternalDHCP4 indicates whether the IPAM is static or allocation by the external DHCP server
	// +optional
	ExternalDHCP4 *bool `json:"externalDHCP4,omitempty"`

	// ParametersRef is a reference to a resource that contains vendor or implementation specific
	// configurations for the network.
	// +optional
	ParametersRef *NetworkParametersReference `json:"parametersRef,omitempty"`
}

// NetworkParametersReference identifies an API object containing additional parameters for the network.
type NetworkParametersReference struct {
	// Group is the API group of k8s resource, e.g. "networking.k8s.io".
	Group string `json:"group"`

	// Kind is kind of the referent, e.g. "networkpolicy".
	Kind string `json:"kind"`

	// Name is the name of the resource object.
	Name string `json:"name"`

	// Namespace is the namespace of the referent. This field is required when referring to a
	// Namespace-scoped resource and MUST be unset when referring to a Cluster-scoped resource.
	// +optional
	Namespace *string `json:"namespace,omitempty"`
}

// DNSConfig defines the DNS configuration of a network.
// The fields follow k8s pod dnsConfig structure:
// https://github.com/kubernetes/kubernetes/blob/master/staging/src/k8s.io/api/core/v1/types.go#L3555
type DNSConfig struct {
	// A list of nameserver IP addresses.
	// Duplicated nameservers will be removed.
	// +required
	// +kubebuilder:validation:MinItems:=1
	Nameservers []string `json:"nameservers"`
	// A list of DNS search domains for host-name lookup.
	// Duplicated search paths will be removed.
	// +optional
	Searches []string `json:"searches,omitempty"`
}

// Route defines a routing table entry to a specific subnetwork.
type Route struct {
	// To defines a destination IPv4 block in CIDR annotation. e.g. 192.168.0.0/24.
	// The CIDR 0.0.0.0/0 will be rejected.
	// +required
	To string `json:"to"`
}

// NetworkStatus contains the status information related to the network.
type NetworkStatus struct{}

// NodeInterfaceMatcher defines criteria to find the matching interface on host networking.
type NodeInterfaceMatcher struct {
	// InterfaceName specifies the interface name to search on the node.
	// +kubebuilder:validation:MinLength=1
	// +optional
	InterfaceName *string `json:"interfaceName,omitempty"`
}

// L2NetworkConfig contains configurations for L2 type network.
type L2NetworkConfig struct {
	// VlanID is the vlan ID used for the network.
	// If unspecified, vlan tagging is not enabled.
	// +optional
	// +kubebuilder:validation:Maximum=4094
	// +kubebuilder:validation:Minimum=1
	VlanID *int32 `json:"vlanID,omitempty"`
	// PrefixLength4 denotes the IPv4 prefix length of the range
	// corresponding to the network. It is used to assign IPs to the pods for
	// multi-networking. This field is required when IPAM is handled internally and dynamically
	// via CCC. It's disallowed for other cases. For static IP, the prefix length is set as
	// part of the address in NetworkInterface object.
	// +optional
	// +kubebuilder:validation:Maximum=32
	// +kubebuilder:validation:Minimum=1
	PrefixLength4 *int32 `json:"prefixLength4,omitempty"`
}

// +genclient
// +genclient:nonNamespaced
// +genclient:onlyVerbs=get
// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// NetworkList contains a list of Network resources.
type NetworkList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	// Items is a slice of Network resources.
	Items []Network `json:"items"`
}
