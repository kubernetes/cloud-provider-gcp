/*
Copyright 2026 The Kubernetes Authors.

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

package ane

import (
	"encoding/json"
)

// State constants for AliasNetworkEndpoint lifecycle.
const (
	StateCreating = "CREATING"
	StateActive   = "ACTIVE"
	StateDeleting = "DELETING"

	DefaultAliasName = "pod-ip"
	DefaultIPVersion = "IPV4"

	SecurityTagInheritanceInherited = "INHERITED"

	// Preview API version header constants for GCE ANE API preview.
	PreviewAPIVersionHeader = "X-Goog-Api-Version"
	PreviewAPIVersion       = "2026-10-01-preview"
)

// ResourceMetadata contains metadata about the API resource, including the resolved API version (AIP-185).
type ResourceMetadata struct {
	APIVersion string `json:"apiVersion,omitempty"`
}

// UnmarshalJSON supports both camelCase (apiVersion) and snake_case (api_version).
func (m *ResourceMetadata) UnmarshalJSON(data []byte) error {
	var aux struct {
		APIVersionCamel string `json:"apiVersion"`
		APIVersionSnake string `json:"api_version"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if aux.APIVersionCamel != "" {
		m.APIVersion = aux.APIVersionCamel
	} else {
		m.APIVersion = aux.APIVersionSnake
	}
	return nil
}

// AliasNetworkEndpoint represents the GCE AliasNetworkEndpoint zonal resource.
type AliasNetworkEndpoint struct {
	Kind                   string            `json:"kind,omitempty"`
	Id                     uint64            `json:"id,string,omitempty"`
	Name                   string            `json:"name,omitempty"`
	Description            string            `json:"description,omitempty"`
	Subnetwork             string            `json:"subnetwork,omitempty"`
	Host                   *Host             `json:"host,omitempty"`
	Aliases                map[string]*Alias `json:"aliases,omitempty"`
	SecurityTagInheritance string            `json:"securityTagInheritance,omitempty"`
	Status                 *Status           `json:"status,omitempty"`
	ResourceMetadata       *ResourceMetadata `json:"resourceMetadata,omitempty"`
	CreationTimestamp      string            `json:"creationTimestamp,omitempty"`
	SelfLink               string            `json:"selfLink,omitempty"`
}

// UnmarshalJSON supports both camelCase (resourceMetadata) and snake_case (resource_metadata).
func (a *AliasNetworkEndpoint) UnmarshalJSON(data []byte) error {
	type Alias AliasNetworkEndpoint
	aux := struct {
		*Alias
		ResourceMetadataSnake *ResourceMetadata `json:"resource_metadata,omitempty"`
	}{
		Alias: (*Alias)(a),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if a.ResourceMetadata == nil && aux.ResourceMetadataSnake != nil {
		a.ResourceMetadata = aux.ResourceMetadataSnake
	}
	return nil
}

// Host represents the binding to a VM instance.
type Host struct {
	Instance string `json:"instance,omitempty"`
}

// Alias represents an IP alias configuration on the endpoint.
type Alias struct {
	IPAddress                    string   `json:"ipAddress,omitempty"`
	IPVersion                    string   `json:"ipVersion,omitempty"`
	SubnetworkRangeNames         []string `json:"subnetworkRangeNames,omitempty"`
	EffectiveIPAddress           string   `json:"effectiveIpAddress,omitempty"`
	EffectiveSubnetworkRangeName string   `json:"effectiveSubnetworkRangeName,omitempty"`
}

// Status represents the lifecycle state of the endpoint.
type Status struct {
	State string `json:"state,omitempty"`
}

// AliasNetworkEndpointList is the paginated response for listing ANEs.
type AliasNetworkEndpointList struct {
	Kind          string                  `json:"kind,omitempty"`
	Items         []*AliasNetworkEndpoint `json:"items,omitempty"`
	NextPageToken string                  `json:"nextPageToken,omitempty"`
}

// ErrorInfo represents structured AIP-193 error details (google.rpc.ErrorInfo).
type ErrorInfo struct {
	Type     string            `json:"@type,omitempty"`
	Reason   string            `json:"reason,omitempty"`
	Domain   string            `json:"domain,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// OperationError contains error details for a failed Operation.
type OperationError struct {
	Errors []struct {
		Code    string `json:"code,omitempty"`
		Message string `json:"message,omitempty"`
	} `json:"errors,omitempty"`
	Details []ErrorInfo `json:"details,omitempty"`
}

// Operation represents the GCE compute#operation returned by mutation endpoints.
type Operation struct {
	Kind                string          `json:"kind,omitempty"`
	Id                  uint64          `json:"id,string,omitempty"`
	Name                string          `json:"name,omitempty"`
	Zone                string          `json:"zone,omitempty"`
	OperationType       string          `json:"operationType,omitempty"`
	Status              string          `json:"status,omitempty"`
	TargetLink          string          `json:"targetLink,omitempty"`
	Error               *OperationError `json:"error,omitempty"`
	HttpErrorMessage   string          `json:"httpErrorMessage,omitempty"`
	HttpErrorStatusCode int             `json:"httpErrorStatusCode,omitempty"`
}

// APIErrorResponse represents standard Google REST API error responses (AIP-193).
type APIErrorResponse struct {
	Error struct {
		Code    int         `json:"code,omitempty"`
		Message string      `json:"message,omitempty"`
		Errors  []struct {
			Code    string `json:"code,omitempty"`
			Message string `json:"message,omitempty"`
			Reason  string `json:"reason,omitempty"`
		} `json:"errors,omitempty"`
		Details []ErrorInfo `json:"details,omitempty"`
	} `json:"error,omitempty"`
}
