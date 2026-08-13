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
	"context"

	"google.golang.org/grpc"
	pb "k8s.io/metis/api/adaptiveipam/v1"
	"k8s.io/metis/pkg/daemon"
)

// directClientAdapter adapts an in-process IPAMEngine instance to satisfy the
// gRPC pb.AdaptiveIpamClient interface without starting any network server or UDS listener.
type directClientAdapter struct {
	engine *daemon.IPAMEngine
}

func (a *directClientAdapter) AllocatePodIP(ctx context.Context, in *pb.AllocatePodIPRequest, _ ...grpc.CallOption) (*pb.AllocatePodIPResponse, error) {
	return a.engine.AllocatePodIP(ctx, in)
}

func (a *directClientAdapter) DeallocatePodIP(ctx context.Context, in *pb.DeallocatePodIPRequest, _ ...grpc.CallOption) (*pb.DeallocatePodIPResponse, error) {
	return a.engine.DeallocatePodIP(ctx, in)
}

func (a *directClientAdapter) CheckPodIP(ctx context.Context, in *pb.CheckPodIPRequest, _ ...grpc.CallOption) (*pb.CheckPodIPResponse, error) {
	return a.engine.CheckPodIP(ctx, in)
}
