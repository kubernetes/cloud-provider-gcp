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

package errors

// MetisErrorDomain is the globally unique domain for Metis error propagation.
// Aligns with Google Cloud GKE container service error specifications.
const MetisErrorDomain = "container.googleapis.com"

// Standard ErrorInfo metadata key constants
const (
	MetadataKeyNetwork      = "network"
	MetadataKeyPodName      = "pod_name"
	MetadataKeyPodNamespace = "pod_namespace"
	MetadataKeyContainerID  = "container_id"
)

// Well-defined error reasons across Metis.
//
// Developer Requirement:
// Every new error reason constant introduced in Metis MUST be defined here and
// registered in reasonToCNIMap in pkg/cni/error_mappings.go with its CNI code,
// default message, root cause, and CNI/Kubelet recovery expectations.
// See reasonToCNIMap in pkg/cni/error_mappings.go for detailed root cause & expectations.

// ReasonNetworkConfigInvalid indicates missing or malformed request parameters
// (gRPC Code: InvalidArgument -> CNI Code 7 ErrInvalidNetworkConfig).
const ReasonNetworkConfigInvalid = "NETWORK_CONFIG_INVALID"

// ReasonInternalError indicates an internal server, database, or uncaught
// component error within Metis (gRPC Code: Internal -> CNI Code 999 ErrInternal).
const ReasonInternalError = "INTERNAL_ERROR"
