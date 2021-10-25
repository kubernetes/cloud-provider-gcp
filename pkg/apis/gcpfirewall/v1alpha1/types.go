/*
Copyright 2021 The Kubernetes Authors.
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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Protocol defines network protocols supported for GCP firewall.
type Protocol string

// CIDR defines a IP block.
// TODO(sugangli) Modify the validation to include IPv6 CIDRs with FW 3.0 support.
// +kubebuilder:validation:Pattern=`^((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)(/(3[0-2]|2[0-9]|1[0-9]|[0-9]))?$`
type CIDR string

// +genclient
// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=gf

// GCPFirewall describes a GCP firewall spec that can be used to configure GCE
// firewalls. A GCPFirewallSpec will correspond 1:1 with a GCE firewall rule.
type GCPFirewall struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the desired configuration for GCP firewall
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	Spec GCPFirewallSpec `json:"spec,omitempty"`

	// Status is the runtime status of this GCP firewall
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	Status GCPFirewallStatus `json:"status,omitempty"`
}

// Action defines the rule action of the firewall rule.
type Action string

const (
	// ActionAllow is the Allow Action of GCP Firewall Rule
	ActionAllow Action = "ALLOW"
	// ActionDeny is the Deny Action of GCP Firewall Rule. For now, only Allow is supported.
	ActionDeny Action = "DENY"
	// ProtocolTCP is the TCP protocol.
	ProtocolTCP Protocol = "TCP"
	// ProtocolUDP is the UDP protocol.
	ProtocolUDP Protocol = "UDP"
	// ProtocolICMP is the ICMP protocol.
	ProtocolICMP Protocol = "ICMP"
)

// GCPFirewallSpec provides the specification of a GCPFirewall.
// The firewall rule apply to the cluster associated targets (network tags or
// secure tags) which are deduced by the controller. As a result, the specified
// rule applies to ALL nodes and pods in the cluster.
type GCPFirewallSpec struct {
	// Rule action of the firewall rule. Only allow action is supported. If not
	// specified, defaults to ALLOW.
	// +optional
	// +kubebuilder:validation:Enum=ALLOW
	// +kubebuilder:default=ALLOW
	Action Action `json:"action"`

	// If set to true, the GCPFirewall is not synced by the controller.
	Disabled bool `json:"disabled,omitempty"`

	// List of protocol/ ports which needs to be selected by this rule.
	// If this field is empty or missing, this rule matches all protocol/ ports.
	// If this field is present and contains at least one item, then this rule
	// allows traffic only if the traffic matches at least one port in the list.
	// +optional
	Ports []ProtocolPort `json:"ports,omitempty"`

	// A collection of sources from which ingress traffic is allowed. If this
	// field is missing, no ingress rules are programmed. if this field is empty,
	// this rule allows all sources.
	// +optional
	Ingress *GCPFirewallIngressPeer `json:"ingress,omitempty"`
}

// GCPFirewallIngressPeer describes a peer to allow traffic from. Items in the
// sub-fields are combined using a logical OR operation. If all the sub-fields
// are missing, then traffic is allowed from all sources.
type GCPFirewallIngressPeer struct {
	// IPBlocks specify the set of CIDRs that the rule applies to. If this field
	// is present and contains at least one item, this rule allows traffic only if
	// the traffic matches at least one item in the list.
	// Valid example list items are "192.168.1.1/24" or "2001:db9::/64".
	// +optional
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=256
	IPBlocks []CIDR `json:"ipBlocks,omitempty"`
}

// ProtocolPort describes the protocol and ports to allow traffic on.
type ProtocolPort struct {
	// The protocol which the traffic must match.
	// +kubebuilder:validation:Enum=TCP;UDP;ICMP;SCTP;AH;ESP
	Protocol Protocol `json:"protocol"`

	// StartPort is the starting port of the port range that is selected on the
	// firewall rule targets for the specified protocol. If EndPort is not
	// specified, this is the only port selected.
	// If StartPort is not provided, all ports are matched.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	StartPort *int32 `json:"startPort,omitempty"`

	// EndPort is the last port of the port range that is selected on the firewall
	// rule targets. If StartPort is not specified or greater than this value, then
	// this field is ignored.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	EndPort *int32 `json:"endPort,omitempty"`
}

// GCPFirewallStatus is the runtime status of a GCP firewall
type GCPFirewallStatus struct {
	// Type specifies the underlying GCE firewall implementation type.
	// Takes one of the values from [VPC, REGIONAL, GLOBAL]
	// +optional
	// +kubebuilder:validation:Enum=VPC;REGIONAL;GLOBAL
	Type string `json:"type,omitempty"`

	// Resource link for the GCE firewall rule. In case of FW 3.0, this is the GCE
	// Network Firewall Policy resource.
	// +optional
	ResourceURL string `json:"resourceURL"`

	// Priority of the GCP firewall rule.
	// +optional
	Priority uint32 `json:"priority"`

	// Conditions describe the current condition of the firewall rule.
	// +optional
	// +listType=map
	// +listMapKey=type
	// +kubebuilder:validation:MaxItems=8
	Conditions []metav1.Condition `json:"conditions"`
}

// FirewallRuleConditionType describes a state of a GCE firewall rule.
type FirewallRuleConditionType string

// FirewallRuleConditionReason specifies the reason for the GCE firewall rule
// to be in the specified state.
type FirewallRuleConditionReason string

const (
	// FirewallRuleConditionEnforced indicates if the firewall rule is enforced.
	FirewallRuleConditionEnforced FirewallRuleConditionType = "Enforced"

	// FirewallRuleReasonInvalid is used when the specified configuration is not valid.
	FirewallRuleReasonInvalid FirewallRuleConditionReason = "Invalid"

	// FirewallRuleReasonSyncError is used if the sync fails due to an error.
	FirewallRuleReasonSyncError FirewallRuleConditionReason = "SyncError"

	// FirewallRuleReasonPending is used when the firewall rule is not synced to
	// GCP and enforced yet.
	FirewallRuleReasonPending FirewallRuleConditionReason = "Pending"
)

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GCPFirewallList contains a list of GCPFirewall resources.
type GCPFirewallList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	// Items is a list of GCP Firewalls.
	Items []GCPFirewall `json:"items"`
}
