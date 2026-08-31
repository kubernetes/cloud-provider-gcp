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
	"net"
	"os"
	"time"

	"github.com/go-logr/logr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
	"k8s.io/metis/api/adaptiveipam/v1"
	adminv1 "k8s.io/metis/api/admin/v1"
	"k8s.io/metis/pkg"
	"k8s.io/metis/pkg/metrics"
	"k8s.io/metis/pkg/store"
)

type adaptiveIpamServer struct {
	adaptiveipam.UnimplementedAdaptiveIpamServer
	adminv1.UnimplementedAdminServer
	engine        *IPAMEngine
	store         *store.Store
	sockPath      string
	grpcServer    *grpc.Server
	logger        logr.Logger
	enableMetrics bool
}

func newAdaptiveIpamServer(logger logr.Logger, storeInstance *store.Store, socketPath string, releaseCooldown time.Duration, busyTimeout time.Duration, enableMetrics bool) *adaptiveIpamServer {
	engine := NewIPAMEngine(logger, storeInstance, releaseCooldown, busyTimeout, nil, enableMetrics)
	return &adaptiveIpamServer{
		engine:        engine,
		store:         storeInstance,
		sockPath:      socketPath,
		logger:        logger,
		enableMetrics: enableMetrics,
	}
}

func (s *adaptiveIpamServer) recordMetrics(method, network, containerID, podName string, err error, start time.Time) {
	if !s.enableMetrics {
		return
	}
	duration := time.Since(start).Seconds()
	code := status.Code(err).String()
	metrics.GRPCServerHandledTotal.WithLabelValues(method, code, network, containerID, podName).Inc()
	metrics.RPCLatencySeconds.WithLabelValues(method, network, containerID, podName).Observe(duration)
}

func (s *adaptiveIpamServer) AllocatePodIP(ctx context.Context, req *adaptiveipam.AllocatePodIPRequest) (*adaptiveipam.AllocatePodIPResponse, error) {
	start := time.Now()
	resp, err := s.engine.AllocatePodIP(ctx, req)
	s.recordMetrics("AllocatePodIP", req.Network, getContainerIDFromAllocate(req), req.PodName, err, start)
	return resp, err
}

func (s *adaptiveIpamServer) DeallocatePodIP(ctx context.Context, req *adaptiveipam.DeallocatePodIPRequest) (*adaptiveipam.DeallocatePodIPResponse, error) {
	start := time.Now()
	resp, err := s.engine.DeallocatePodIP(ctx, req)
	s.recordMetrics("DeallocatePodIP", req.Network, req.ContainerId, req.PodName, err, start)
	return resp, err
}

func (s *adaptiveIpamServer) CheckPodIP(ctx context.Context, req *adaptiveipam.CheckPodIPRequest) (*adaptiveipam.CheckPodIPResponse, error) {
	start := time.Now()
	resp, err := s.engine.CheckPodIP(ctx, req)
	s.recordMetrics("CheckPodIP", req.Network, req.ContainerId, req.PodName, err, start)
	return resp, err
}

func getContainerIDFromAllocate(req *adaptiveipam.AllocatePodIPRequest) string {
	if req == nil {
		return ""
	}
	if req.Ipv4Config != nil && req.Ipv4Config.ContainerId != "" {
		return req.Ipv4Config.ContainerId
	}
	if req.Ipv6Config != nil && req.Ipv6Config.ContainerId != "" {
		return req.Ipv6Config.ContainerId
	}
	return ""
}

func (s *adaptiveIpamServer) getPendingRequestsCount(network string) int {
	return s.engine.getPendingRequestsCount(network)
}

func (s *adaptiveIpamServer) onCIDRAdded(network string, availableIPs int) {
	s.engine.onCIDRAdded(network, availableIPs)
}

func (s *adaptiveIpamServer) start() error {
	sockPath := s.sockPath
	if sockPath == "" {
		sockPath = pkg.DefaultSockPath
	}

	if err := os.Remove(sockPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove existing socket: %w", err)
	}

	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		return fmt.Errorf("failed to listen on uds %s: %w", sockPath, err)
	}
	defer listener.Close()

	// Explicitly restrict socket permissions to owner-only (0600) to prevent
	// unauthorized local processes from interacting with the daemon.
	if err := os.Chmod(sockPath, 0600); err != nil {
		return fmt.Errorf("failed to set permissions on socket %s: %w", sockPath, err)
	}

	s.grpcServer = grpc.NewServer()
	adaptiveipam.RegisterAdaptiveIpamServer(s.grpcServer, s)
	adminv1.RegisterAdminServer(s.grpcServer, s)

	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(s.grpcServer, healthServer)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	s.logger.Info("gRPC server is listening", "socket", sockPath)
	return s.grpcServer.Serve(listener)
}

func (s *adaptiveIpamServer) stop() {
	if s.grpcServer != nil {
		s.logger.Info("Stopping gRPC server gracefully")
		s.grpcServer.GracefulStop()
	}
}
