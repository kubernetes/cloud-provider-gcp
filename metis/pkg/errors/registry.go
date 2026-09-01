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
	"google.golang.org/grpc/codes"
)

// MetisErrorDomain is the globally unique domain for Metis error propagation.
// Aligns with Google Cloud GKE container service error specifications.
const MetisErrorDomain = "container.googleapis.com"

// ReasonSpec encapsulates the machine-readable reason string, gRPC status code,
// CNI specification code, and user-facing error message for a Metis error.
type ReasonSpec struct {
	Reason   string
	GRPCCode codes.Code
	CNICode  uint
	Msg      string
}

var reasonMap map[string]ReasonSpec

func init() {
	allSpecs := []ReasonSpec{
		ReasonNetworkConfigInvalid,
		ReasonInternalError,
	}
	reasonMap = make(map[string]ReasonSpec, len(allSpecs))
	for _, spec := range allSpecs {
		reasonMap[spec.Reason] = spec
	}
}

// LookupReason provides O(1) constant-time lookup for gRPC wire string payloads.
func LookupReason(reason string) (ReasonSpec, bool) {
	spec, ok := reasonMap[reason]
	return spec, ok
}
