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

func TestErrIPPoolExhausted(t *testing.T) {
	causeErr := errors.New("underlying pool empty")
	podInfo := PodInfo{
		PodName:      "nginx-pod",
		PodNamespace: "production",
		ContainerID:  "container-123",
		Network:      "test-network",
	}
	mErr := ErrIPPoolExhausted(podInfo, causeErr)

	if mErr.GRPCCode() != codes.ResourceExhausted {
		t.Errorf("expected GRPCCode %v, got %v", codes.ResourceExhausted, mErr.GRPCCode())
	}

	if mErr.Reason() != ReasonIPPoolExhausted {
		t.Errorf("expected Reason %q, got %q", ReasonIPPoolExhausted, mErr.Reason())
	}

	if mErr.Metadata()[MetadataKeyNetwork] != "test-network" {
		t.Errorf("expected metadata network 'test-network', got %q", mErr.Metadata()[MetadataKeyNetwork])
	}

	if mErr.Metadata()[MetadataKeyPodName] != "nginx-pod" {
		t.Errorf("expected metadata pod_name 'nginx-pod', got %q", mErr.Metadata()[MetadataKeyPodName])
	}

	if mErr.Metadata()[MetadataKeyPodNamespace] != "production" {
		t.Errorf("expected metadata pod_namespace 'production', got %q", mErr.Metadata()[MetadataKeyPodNamespace])
	}

	if !errors.Is(mErr.Unwrap(), causeErr) {
		t.Errorf("expected cause error %v, got %v", causeErr, mErr.Unwrap())
	}

	st := mErr.ToGRPCStatus()
	if st.Code() != codes.ResourceExhausted {
		t.Errorf("expected status code %v, got %v", codes.ResourceExhausted, st.Code())
	}

	var foundErrorInfo bool
	for _, detail := range st.Details() {
		if errorInfo, ok := detail.(*genproto.ErrorInfo); ok {
			foundErrorInfo = true
			if errorInfo.Reason != ReasonIPPoolExhausted {
				t.Errorf("expected ErrorInfo reason %q, got %q", ReasonIPPoolExhausted, errorInfo.Reason)
			}
			if errorInfo.Domain != MetisErrorDomain {
				t.Errorf("expected ErrorInfo domain %q, got %q", MetisErrorDomain, errorInfo.Domain)
			}
			if errorInfo.Metadata[MetadataKeyNetwork] != "test-network" {
				t.Errorf("expected ErrorInfo metadata 'test-network', got %q", errorInfo.Metadata[MetadataKeyNetwork])
			}
			if errorInfo.Metadata[MetadataKeyPodName] != "nginx-pod" {
				t.Errorf("expected ErrorInfo metadata pod_name 'nginx-pod', got %q", errorInfo.Metadata[MetadataKeyPodName])
			}
		}
	}

	if !foundErrorInfo {
		t.Errorf("expected ErrorInfo detail in status, none found")
	}
}

func TestMetisErrorAs(t *testing.T) {
	mErr := ErrIPPoolExhausted(PodInfo{Network: "default"}, nil)
	target, ok := errors.AsType[MetisError](mErr)
	if !ok {
		t.Fatalf("errors.AsType failed to match MetisError interface")
	}
	if target.Reason() != ReasonIPPoolExhausted {
		t.Errorf("expected reason %q, got %q", ReasonIPPoolExhausted, target.Reason())
	}
}
