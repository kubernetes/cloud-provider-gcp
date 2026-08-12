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
	"testing"

	"github.com/containernetworking/cni/pkg/types"

	metiserrors "k8s.io/metis/pkg/errors"
)

func TestToCNIError_IPPoolExhausted(t *testing.T) {
	mErr := metiserrors.ErrIPPoolExhausted("test-net", errors.New("empty"))
	grpcErr := mErr.ToGRPCStatus().Err()

	cniErr := ToCNIError(grpcErr, "cni add failed")

	if cniErr.Code != types.ErrTryAgainLater {
		t.Errorf("expected CNI Code %v, got %v", types.ErrTryAgainLater, cniErr.Code)
	}

	if cniErr.Msg != "IP address pool exhausted" {
		t.Errorf("expected Msg 'IP address pool exhausted', got %q", cniErr.Msg)
	}
}

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
