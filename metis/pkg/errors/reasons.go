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

import (
	"github.com/containernetworking/cni/pkg/types"
	"google.golang.org/grpc/codes"
)

// Well-defined error reason specifications across Metis.
var (
	// ReasonNetworkConfigInvalid (NETWORK_CONFIG_INVALID): missing or malformed request parameters.
	//
	// Root Cause:
	// - CNI netconf JSON stdin parameters or AllocatePodIP gRPC request fields missing or malformed.
	//
	// CNI / Kubelet Expectation:
	// - Non-retriable Pod sandbox creation failure (CNI Code 7 ErrInvalidNetworkConfig).
	// - Kubelet/containerd stops retrying Pod setup for this container until configuration is corrected.
	ReasonNetworkConfigInvalid = ReasonSpec{
		Reason:   "NETWORK_CONFIG_INVALID",
		GRPCCode: codes.InvalidArgument,
		CNICode:  types.ErrInvalidNetworkConfig,
		Msg:      "invalid request parameters",
	}

	// ReasonInternalError (INTERNAL_ERROR): internal server, database, or uncaught component error.
	//
	// Root Cause:
	// - SQLite database transaction failures, disk I/O errors, uncaught daemon panics, or unhandled component errors.
	//
	// CNI / Kubelet Expectation:
	// - Terminal daemon component error for current allocation attempt (CNI Code 999 ErrInternal).
	// - Kubelet logs error details. Subsequent allocation attempts may succeed if daemon recovers.
	ReasonInternalError = ReasonSpec{
		Reason:   "INTERNAL_ERROR",
		GRPCCode: codes.Internal,
		CNICode:  types.ErrInternal,
		Msg:      "internal daemon error",
	}
)
