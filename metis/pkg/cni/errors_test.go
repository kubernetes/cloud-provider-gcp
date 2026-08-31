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
	"errors"
	"strings"
	"testing"

	"github.com/containernetworking/cni/pkg/types"

	metiserrors "k8s.io/metis/pkg/errors"
)

// TestToCNIError_NetworkConfigInvalid tests that ToCNIError maps a gRPC status
// containing ReasonNetworkConfigInvalid to CNI Code 7 (ErrInvalidNetworkConfig).
func TestToCNIError_NetworkConfigInvalid(t *testing.T) {
	meta := map[string]string{
		metiserrors.MetadataKeyNetwork:      "default",
		metiserrors.MetadataKeyPodName:      "test-pod",
		metiserrors.MetadataKeyPodNamespace: "production",
	}
	mErr := metiserrors.ErrNetworkConfigInvalid("missing network", meta, errors.New("invalid"))
	grpcErr := mErr.ToGRPCStatus().Err()

	cniErr := ToCNIError(grpcErr, "cni add failed")

	if cniErr.Code != types.ErrInvalidNetworkConfig {
		t.Errorf("expected CNI Code %v, got %v", types.ErrInvalidNetworkConfig, cniErr.Code)
	}

	if cniErr.Msg != "invalid request parameters" {
		t.Errorf("expected Msg 'invalid request parameters', got %q", cniErr.Msg)
	}

	if !strings.Contains(cniErr.Details, "Metadata:") || !strings.Contains(cniErr.Details, "test-pod") {
		t.Errorf("expected Details to contain metadata and test-pod, got %q", cniErr.Details)
	}
}

// TestToCNIError_InternalError tests that ToCNIError maps a gRPC status
// containing ReasonInternalError to CNI Code 999 (ErrInternal).
func TestToCNIError_InternalError(t *testing.T) {
	mErr := metiserrors.ErrInternal("db write failed", errors.New("disk full"))
	grpcErr := mErr.ToGRPCStatus().Err()

	cniErr := ToCNIError(grpcErr, "cni add failed")

	if cniErr.Code != types.ErrInternal {
		t.Errorf("expected CNI Code %v, got %v", types.ErrInternal, cniErr.Code)
	}

	if cniErr.Msg != "internal daemon error" {
		t.Errorf("expected Msg 'internal daemon error', got %q", cniErr.Msg)
	}
}

// TestToCNIError_FallbackGenericError tests that ToCNIError maps generic
// non-Metis errors to CNI Code 999 (ErrInternal).
func TestToCNIError_FallbackGenericError(t *testing.T) {
	genericErr := errors.New("random disk error")
	cniErr := ToCNIError(genericErr, "cni add failed")

	if cniErr.Code != types.ErrInternal {
		t.Errorf("expected CNI Code %v, got %v", types.ErrInternal, cniErr.Code)
	}

	if cniErr.Details != "random disk error" {
		t.Errorf("expected Details 'random disk error', got %q", cniErr.Details)
	}
}
