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
	"net"
	"os"
	"time"

	"github.com/go-logr/logr"
	"google.golang.org/grpc"
	"k8s.io/metis/api/adaptiveipam/v1"
	adminv1 "k8s.io/metis/api/admin/v1"
	"k8s.io/metis/pkg"
	metiserrors "k8s.io/metis/pkg/errors"
	"k8s.io/metis/pkg/store"
)

type adaptiveIpamServer struct {
	adaptiveipam.UnimplementedAdaptiveIpamServer
	adminv1.UnimplementedAdminServer
	engine     *IPAMEngine
	store      *store.Store
	sockPath   string
	grpcServer *grpc.Server
	logger     logr.Logger
}

func newAdaptiveIpamServer(logger logr.Logger, storeInstance *store.Store, socketPath string, releaseCooldown time.Duration, busyTimeout time.Duration) *adaptiveIpamServer {
	engine := NewIPAMEngine(logger, storeInstance, releaseCooldown, busyTimeout, nil)
	return &adaptiveIpamServer{
		engine:   engine,
		store:    storeInstance,
		sockPath: socketPath,
		logger:   logger,
	}
}

func (s *adaptiveIpamServer) AllocatePodIP(ctx context.Context, req *adaptiveipam.AllocatePodIPRequest) (*adaptiveipam.AllocatePodIPResponse, error) {
	return s.engine.AllocatePodIP(ctx, req)
}

func (s *adaptiveIpamServer) DeallocatePodIP(ctx context.Context, req *adaptiveipam.DeallocatePodIPRequest) (*adaptiveipam.DeallocatePodIPResponse, error) {
	return s.engine.DeallocatePodIP(ctx, req)
}

func (s *adaptiveIpamServer) CheckPodIP(ctx context.Context, req *adaptiveipam.CheckPodIPRequest) (*adaptiveipam.CheckPodIPResponse, error) {
	return s.engine.CheckPodIP(ctx, req)
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

	s.grpcServer = grpc.NewServer(grpc.UnaryInterceptor(s.ErrorInterceptor))
	adaptiveipam.RegisterAdaptiveIpamServer(s.grpcServer, s)
	adminv1.RegisterAdminServer(s.grpcServer, s)

	s.logger.Info("gRPC server is listening", "socket", sockPath)
	return s.grpcServer.Serve(listener)
}

func (s *adaptiveIpamServer) ErrorInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	resp, err := handler(ctx, req)
	if err == nil {
		return resp, nil
	}

	var mErr metiserrors.MetisError
	if errors.As(err, &mErr) {
		if mErr.Unwrap() != nil {
			s.logger.V(4).Info("Domain error cause details", "method", info.FullMethod, "reason", mErr.Reason(), "cause", mErr.Unwrap())
		}
		return nil, mErr.ToGRPCStatus().Err()
	}

	return nil, err
}

func (s *adaptiveIpamServer) stop() {
	if s.grpcServer != nil {
		s.logger.Info("Stopping gRPC server gracefully")
		s.grpcServer.GracefulStop()
	}
}
