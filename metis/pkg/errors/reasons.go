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

// ReasonIPPoolExhausted indicates that no available IP addresses remain in the
// requested network subnet pool for pod allocation.
//
// Semantics:
// - Triggered during Pod IP allocation when the IPAM subnet pool capacity is exhausted.
// - gRPC Code: ResourceExhausted (14).
// - CNI Mapping: Code 11 (ErrTryAgainLater).
//
// Remediation:
// The caller (CNI plugin / kubelet) should retry with exponential backoff
// or wait until existing pod network interfaces on the node are released.
const ReasonIPPoolExhausted = "IP_POOL_EXHAUSTED"
