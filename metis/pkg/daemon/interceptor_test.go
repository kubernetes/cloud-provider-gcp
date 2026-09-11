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
	"testing"

	"github.com/go-logr/logr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/metis/api/adaptiveipam/v1"
	"k8s.io/metis/pkg/metrics"
)

func TestMetricsUnaryInterceptor(t *testing.T) {
	interceptor := metricsUnaryInterceptor(metrics.NewNoOpRecorder(), logr.Discard())

	t.Run("nil request handling", func(t *testing.T) {
		handler := func(_ context.Context, _ any) (any, error) {
			return nil, status.Error(codes.InvalidArgument, "request cannot be nil")
		}
		info := &grpc.UnaryServerInfo{FullMethod: "/adaptiveipam.v1.AdaptiveIpam/AllocatePodIP"}
		_, err := interceptor(context.Background(), nil, info, handler)
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("Expected InvalidArgument, got %v", status.Code(err))
		}
	})

	t.Run("panicking handler recovery", func(t *testing.T) {
		handler := func(_ context.Context, _ any) (any, error) {
			panic("simulated handler panic")
		}
		info := &grpc.UnaryServerInfo{FullMethod: "/adaptiveipam.v1.AdaptiveIpam/AllocatePodIP"}
		_, err := interceptor(context.Background(), nil, info, handler)
		if status.Code(err) != codes.Internal {
			t.Errorf("Expected Internal status code for panic recovery, got %v", status.Code(err))
		}
	})

	t.Run("extract metadata", func(t *testing.T) {
		req := &adaptiveipam.AllocatePodIPRequest{
			Network: "test-net",
			PodName: "test-pod",
			Ipv4Config: &adaptiveipam.IPConfig{
				ContainerId: "cid-123",
			},
		}
		net, cid, pod := extractReqMetadata(req)
		if net != "test-net" || cid != "cid-123" || pod != "test-pod" {
			t.Errorf("Unexpected metadata: net=%q, cid=%q, pod=%q", net, cid, pod)
		}
	})
}
