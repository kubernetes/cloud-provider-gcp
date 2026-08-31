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

// Well-defined error reasons across Metis

// ReasonNetworkConfigInvalid indicates that request parameters or CNI stdin
// netconf JSON parameters are invalid or missing.
//
// Semantics:
//   - Triggered during Pod IP allocation or CNI execution when required fields
//     (e.g. network name, pod configs) are missing or malformed.
//   - gRPC Code: InvalidArgument (3).
//   - CNI Mapping: Code 7 (ErrInvalidNetworkConfig).
const ReasonNetworkConfigInvalid = "NETWORK_CONFIG_INVALID"

// ReasonInternalError indicates an internal server, database, or uncaught
// component error within Metis.
//
// Semantics:
// - Triggered when SQLite database transactions fail or uncaught panics occur.
// - gRPC Code: Internal (13).
// - CNI Mapping: Code 999 (ErrInternal).
const ReasonInternalError = "INTERNAL_ERROR"
