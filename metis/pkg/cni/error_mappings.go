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

package cni

import (
	"github.com/containernetworking/cni/pkg/types"

	metiserrors "k8s.io/metis/pkg/errors"
)

// cniMapping encapsulates the target CNI error code and user-facing default
// error message for a given machine-readable Metis error reason.
type cniMapping struct {
	code       uint
	defaultMsg string
}

// reasonToCNIMap is the declarative lookup table mapping machine-readable
// Metis error reasons (from google.rpc.ErrorInfo) to CNI specification codes.
//
// Every new Metis error reason introduced across Metis MUST be registered in this
// mapping table with a valid CNI specification error code, default message, and
// documentation explaining root causes and CNI/Kubelet expectations.
var reasonToCNIMap = map[string]cniMapping{
	// ReasonNetworkConfigInvalid (NETWORK_CONFIG_INVALID) maps to CNI Code 7 (ErrInvalidNetworkConfig).
	//
	// Root Cause:
	// - Triggered when CNI netconf JSON stdin parameters or AllocatePodIP gRPC
	//   request fields (e.g. missing network name, missing pod IP configs, or
	//   empty container_id / interface_name) are missing or malformed.
	//
	// CNI / Kubelet Expectation:
	// - Non-retriable Pod sandbox creation failure.
	// - Kubelet/containerd MUST stop retrying sandbox setup for this container
	//   and mark the Pod setup as failed until configuration is corrected.
	metiserrors.ReasonNetworkConfigInvalid: {
		code:       types.ErrInvalidNetworkConfig, // CNI Code 7
		defaultMsg: "invalid request parameters",
	},

	// ReasonInternalError (INTERNAL_ERROR) maps to CNI Code 999 (ErrInternal).
	//
	// Root Cause:
	// - Triggered by SQLite database transaction failures, disk I/O errors,
	//   uncaught daemon panics, or unhandled internal component errors.
	//
	// CNI / Kubelet Expectation:
	// - Terminal daemon component error for the current allocation attempt.
	// - Kubelet/containerd logs the error details. If daemon recovers or restarts,
	//   subsequent allocation attempts may succeed; persistent errors require operator
	//   inspection of daemon health.
	metiserrors.ReasonInternalError: {
		code:       types.ErrInternal, // CNI Code 999
		defaultMsg: "internal daemon error",
	},
}

// LookupCNIMapping retrieves the CNI error code and default message for a
// given Metis error reason. Returns false if the reason is unmapped.
func LookupCNIMapping(reason string) (uint, string, bool) {
	mapping, ok := reasonToCNIMap[reason]
	if !ok {
		return 0, "", false
	}
	return mapping.code, mapping.defaultMsg, true
}
