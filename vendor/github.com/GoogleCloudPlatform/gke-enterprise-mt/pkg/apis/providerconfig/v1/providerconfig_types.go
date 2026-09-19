package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// ProviderConfig defines an instance of the infrastructure configuration for a GKE Multi-tenant cluster.
//
// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// (-- LINT.IfChange --)
type ProviderConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProviderConfigSpec   `json:"spec"`
	Status ProviderConfigStatus `json:"status,omitempty"`
}

// ProviderConfigList contains a list of ProviderConfigs.
//
// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ProviderConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ProviderConfig `json:"items"`
}

// ProviderNetworkConfig defines the network configuration.
type ProviderNetworkConfig struct {
	// Network is the name of the VPC in the format projects/{project}/global/networks/{networkName}.
	// Format is `projects/{tenantProject}/global/networks/{network}`.
	Network string `json:"network"`
	// SubnetInfo contains information about a specific Subnet within the VPC.
	SubnetInfo ProviderConfigSubnetInfo `json:"subnetInfo"`
}

// ProviderConfigSubnetInfo defines the subnet configuration.
type ProviderConfigSubnetInfo struct {
	// Subnetwork is the name of the subnetwork in the format projects/{project}/regions/{region}/subnetworks/{subnet}.
	Subnetwork string `json:"subnetwork"`
	// The primary IP range of the subnet in CIDR notation (e.g.,`10.0.0.0/16`).
	CIDR string `json:"cidr"`

	// Node IP address ranges in CIDR notation. This is the replacement for CIDR field above to better support dual stack clusters.
	//
	// +kubebuilder:validation:Optional
	NodeCIDRs *Range `json:"nodeCIDRs,omitempty"`

	// PodRanges contains the Pod CIDR ranges that are part of this Subnet.
	PodRanges []ProviderConfigSecondaryRange `json:"podRanges"`
}

// Range describes the configuration of an IP range.
type Range struct {
	// The IPv4 range in CIDR notation (e.g.,`10.0.0.0/16`).
	CIDR string `json:"cidr,omitempty"`
	// The IPv6 range in CIDR notation (e.g.,`fd00::/96`).
	IPv6CIDR string `json:"ipv6CIDR,omitempty"`
}

// ProviderConfigSecondaryRange describes the configuration of a SecondaryRange.
type ProviderConfigSecondaryRange struct {
	// The name of the secondary range.
	Name string `json:"name"`

	Range `json:",inline"`
}

// ProviderConfigSpec defines the desired state of ProviderConfig.
type ProviderConfigSpec struct {
	// ProjectNumber is the GCP project number.
	//
	// +kubebuilder:validation:Minimum=0
	// +(Validation done in accordance with go/elysium/project_ids#project-number)
	ProjectNumber int64 `json:"projectNumber"`
	// ProjectID is the GCP Project ID.
	//
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=30
	// +kubebuilder:validation:Pattern=`^[a-z][a-z0-9-]{4,28}[a-z0-9]$`
	// +(Validation done in accordance with https://cloud.google.com/resource-manager/docs/creating-managing-projects#before_you_begin)
	ProjectID string `json:"projectID"`
	// PSC connection ID of the PSC endpoint.
	//
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Optional
	PSCConnectionID int64                 `json:"pscConnectionID"`
	NetworkConfig   ProviderNetworkConfig `json:"networkConfig"`

	// AuthConfig specifies the configuration for controllers to obtain an authentication token.
	// +kubebuilder:validation:Optional
	AuthConfig *AuthConfig `json:"authSpec,omitempty"`

	// PrincipalInfo contains information about the principal entity associated with this configuration.
	// +kubebuilder:validation:Optional
	PrincipalInfo *PrincipalInfo `json:"principalInfo,omitempty"`
}

// ProviderConfigStatus defines the current state of ProviderConfig.
type ProviderConfigStatus struct {
	// Conditions describe the current conditions of the ProviderConfig.
	//
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// AuthConfig defines the necessary information for a controller to acquire an authentication token.
// This structure is analogous to the token-url and token-body configuration in gce.conf.
type AuthConfig struct {
	// TokenURL is the full URL endpoint for generating an authentication token.
	// +kubebuilder:validation:MinLength=1
	TokenURL string `json:"tokenURL"`

	// TokenBody is the JSON body required for the token generation POST request.
	// +kubebuilder:validation:MinLength=1
	TokenBody string `json:"tokenBody"`
}

// PrincipalInfo defines the identity associated with a ProviderConfig (usually Cluster or Tenant),
// This information is sourced from the corresponding Tenant CR or the cluster configuration.
type PrincipalInfo struct {
	// A unique and stable identifier for the principal.
	// Usecases include: adding tags to the metrics and logs, naming GCP resources.
	// +kubebuilder:validation:MinLength=1
	ID string `json:"id"`
	// A human-friendly name for the principal.
	// Usecases include: adding tags to the metrics and logs, naming GCP resources.
	// +kubebuilder:validation:Optional
	Name string `json:"name,omitempty"`
}

// (-- LINT.ThenChange(//depot/google3/cloud/kubernetes/tenancy/apis/providerconfig/v1/providerconfig_types.go) --)
