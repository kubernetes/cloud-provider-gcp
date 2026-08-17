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

package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	metiserrors "k8s.io/metis/pkg/errors"
)

func TestErrorInterceptor_MetisError(t *testing.T) {
	server := &adaptiveIpamServer{logger: logr.Discard()}

	dummyHandler := func(ctx context.Context, req any) (any, error) {
		podInfo := metiserrors.PodInfo{
			PodName:      "test-pod",
			PodNamespace: "default",
			ContainerID:  "cont-1",
			Network:      "default",
		}
		return nil, metiserrors.ErrIPPoolExhausted(podInfo, errors.New("underlying pool empty"))
	}

	info := &grpc.UnaryServerInfo{FullMethod: "/adaptiveipam.v1.AdaptiveIpam/AllocatePodIP"}
	_, err := server.ErrorInterceptor(context.Background(), nil, info, dummyHandler)

	if err == nil {
		t.Fatal("Expected error from ErrorInterceptor, got nil")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("Expected gRPC status error, got: %v", err)
	}

	if st.Code() != codes.ResourceExhausted {
		t.Errorf("Expected status code ResourceExhausted (8), got %v", st.Code())
	}
}

func TestErrorInterceptor_NonMetisError(t *testing.T) {
	server := &adaptiveIpamServer{logger: logr.Discard()}

	dummyHandler := func(ctx context.Context, req any) (any, error) {
		return nil, fmt.Errorf("bare internal database error")
	}

	info := &grpc.UnaryServerInfo{FullMethod: "/adaptiveipam.v1.AdaptiveIpam/AllocatePodIP"}
	_, err := server.ErrorInterceptor(context.Background(), nil, info, dummyHandler)

	if err == nil {
		t.Fatal("Expected error from ErrorInterceptor, got nil")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("Expected gRPC status error, got: %v", err)
	}

	if st.Code() != codes.Internal {
		t.Errorf("Expected status code Internal (13), got %v", st.Code())
	}

	if !strings.Contains(st.Message(), "bare internal database error") {
		t.Errorf("Expected error message to contain 'bare internal database error', got %q", st.Message())
	}
}
