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
	"fmt"
	"path"
	"time"

	"github.com/go-logr/logr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/metis/api/adaptiveipam/v1"
	"k8s.io/metis/pkg/metrics"
)

func metricsUnaryInterceptor(recorder metrics.MetricsRecorder, logger logr.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		start := time.Now()
		defer func() {
			if r := recover(); r != nil {
				logger.Error(fmt.Errorf("panic recovered: %v", r), "gRPC method panicked", "method", info.FullMethod)
				err = status.Errorf(codes.Internal, "internal server panic: %v", r)
			}
			duration := time.Since(start)
			method := path.Base(info.FullMethod)
			network, containerID, podName := extractReqMetadata(req)
			recorder.RecordGRPCRequest(method, network, containerID, podName, err, duration)
		}()

		resp, err = handler(ctx, req)
		return resp, err
	}
}

func extractReqMetadata(req any) (network, containerID, podName string) {
	if req == nil {
		return "", "", ""
	}
	switch r := req.(type) {
	case *adaptiveipam.AllocatePodIPRequest:
		if r != nil {
			network = r.Network
			podName = r.PodName
			containerID = getContainerIDFromAllocate(r)
		}
	case *adaptiveipam.DeallocatePodIPRequest:
		if r != nil {
			network = r.Network
			containerID = r.ContainerId
			podName = r.PodName
		}
	case *adaptiveipam.CheckPodIPRequest:
		if r != nil {
			network = r.Network
			containerID = r.ContainerId
			podName = r.PodName
		}
	}
	return network, containerID, podName
}
