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
	"errors"
	"testing"

	genproto "google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
)

// TestErrNetworkConfigInvalid tests that ErrNetworkConfigInvalid constructs a
// MetisError with codes.InvalidArgument, ReasonNetworkConfigInvalid, and
// serializes ErrorInfo details into a gRPC Status.
func TestErrNetworkConfigInvalid(t *testing.T) {
	causeErr := errors.New("missing network")
	meta := map[string]string{
		MetadataKeyNetwork:      "test-net",
		MetadataKeyPodName:      "test-pod",
		MetadataKeyPodNamespace: "default",
	}
	mErr := ErrNetworkConfigInvalid("network is required", meta, causeErr)

	if mErr.GRPCCode() != codes.InvalidArgument {
		t.Errorf("expected GRPCCode %v, got %v", codes.InvalidArgument, mErr.GRPCCode())
	}

	if mErr.Reason() != ReasonNetworkConfigInvalid.Reason {
		t.Errorf("expected Reason %q, got %q", ReasonNetworkConfigInvalid.Reason, mErr.Reason())
	}

	if mErr.Metadata()[MetadataKeyPodName] != "test-pod" {
		t.Errorf("expected Metadata pod_name %q, got %q", "test-pod", mErr.Metadata()[MetadataKeyPodName])
	}

	if !errors.Is(mErr.Unwrap(), causeErr) {
		t.Errorf("expected cause error %v, got %v", causeErr, mErr.Unwrap())
	}

	st := mErr.ToGRPCStatus()
	if st.Code() != codes.InvalidArgument {
		t.Errorf("expected status code %v, got %v", codes.InvalidArgument, st.Code())
	}

	var foundErrorInfo bool
	for _, detail := range st.Details() {
		if errorInfo, ok := detail.(*genproto.ErrorInfo); ok {
			foundErrorInfo = true
			if errorInfo.Reason != ReasonNetworkConfigInvalid.Reason {
				t.Errorf("expected ErrorInfo reason %q, got %q", ReasonNetworkConfigInvalid.Reason, errorInfo.Reason)
			}
			if errorInfo.Domain != MetisErrorDomain {
				t.Errorf("expected ErrorInfo domain %q, got %q", MetisErrorDomain, errorInfo.Domain)
			}
			if errorInfo.Metadata[MetadataKeyNetwork] != "test-net" {
				t.Errorf("expected ErrorInfo metadata network %q, got %q", "test-net", errorInfo.Metadata[MetadataKeyNetwork])
			}
		}
	}

	if !foundErrorInfo {
		t.Errorf("expected ErrorInfo detail in status, none found")
	}
}

// TestErrInternal tests that ErrInternal constructs a MetisError with
// codes.Internal, ReasonInternalError, and serializes ErrorInfo.
func TestErrInternal(t *testing.T) {
	causeErr := errors.New("db disk I/O error")
	mErr := ErrInternal("sqlite transaction failed", causeErr)

	if mErr.GRPCCode() != codes.Internal {
		t.Errorf("expected GRPCCode %v, got %v", codes.Internal, mErr.GRPCCode())
	}

	if mErr.Reason() != ReasonInternalError.Reason {
		t.Errorf("expected Reason %q, got %q", ReasonInternalError.Reason, mErr.Reason())
	}

	if !errors.Is(mErr.Unwrap(), causeErr) {
		t.Errorf("expected cause error %v, got %v", causeErr, mErr.Unwrap())
	}

	st := mErr.ToGRPCStatus()
	if st.Code() != codes.Internal {
		t.Errorf("expected status code %v, got %v", codes.Internal, st.Code())
	}
}

// TestMetisErrorAs tests that generic errors.AsType[MetisError] successfully
// extracts structured MetisError instances.
func TestMetisErrorAs(t *testing.T) {
	mErr := ErrNetworkConfigInvalid("invalid config", nil, nil)
	target, ok := errors.AsType[MetisError](mErr)
	if !ok {
		t.Fatalf("errors.AsType failed to match MetisError interface")
	}
	if target.Reason() != ReasonNetworkConfigInvalid.Reason {
		t.Errorf("expected reason %q, got %q", ReasonNetworkConfigInvalid.Reason, target.Reason())
	}
}
