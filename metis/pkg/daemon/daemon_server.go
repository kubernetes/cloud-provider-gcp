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
	"strings"
	"time"

	"github.com/go-logr/logr"
	"google.golang.org/grpc"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/metis/api/adaptiveipam/v1"
	"k8s.io/metis/pkg"
	"k8s.io/metis/pkg/store"
)

type adaptiveIpamServer struct {
	adaptiveipam.UnimplementedAdaptiveIpamServer
	store           *store.Store
	sockPath        string
	releaseCooldown time.Duration
	busyTimeout     time.Duration
	grpcServer      *grpc.Server
	logger          logr.Logger
}

func newAdaptiveIpamServer(logger logr.Logger, storeInstance *store.Store, socketPath string, releaseCooldown time.Duration, busyTimeout time.Duration) *adaptiveIpamServer {
	server := &adaptiveIpamServer{
		store:           storeInstance,
		sockPath:        socketPath,
		releaseCooldown: releaseCooldown,
		busyTimeout:     busyTimeout,
		logger:          logger,
	}

	return server
}

func (s *adaptiveIpamServer) AllocatePodIP(ctx context.Context, req *adaptiveipam.AllocatePodIPRequest) (*adaptiveipam.AllocatePodIPResponse, error) {
	s.logger.Info("AllocatePodIP request received",
		"network", req.Network,
		"podName", req.PodName,
		"podNamespace", req.PodNamespace,
		"ipv4Config", fmt.Sprintf("%+v", req.Ipv4Config),
		"ipv6Config", fmt.Sprintf("%+v", req.Ipv6Config))

	if req.Ipv4Config == nil && req.Ipv6Config == nil {
		err := fmt.Errorf("both ipv4_config and ipv6_config are missing for pod %s/%s", req.PodNamespace, req.PodName)
		s.logger.Error(err, "AllocatePodIP validation failed", "podName", req.PodName, "podNamespace", req.PodNamespace)
		return nil, err
	}

	var ipv4Alloc *adaptiveipam.PodIP
	if req.Ipv4Config != nil {
		if req.Ipv4Config.InitialPodCidr != "" {
			exists, err := s.store.GetCIDRBlockByCIDR(ctx, req.Ipv4Config.InitialPodCidr)
			if err != nil {
				s.logger.Error(err, "failed to check if initial cidr block exists", "network", req.Network, "cidr", req.Ipv4Config.InitialPodCidr)
				return nil, fmt.Errorf("failed to check if initial cidr block %s exists for network %s: %w", req.Ipv4Config.InitialPodCidr, req.Network, err)
			}
			if !exists {
				if err := s.store.AddCIDR(ctx, req.Network, req.Ipv4Config.InitialPodCidr); err != nil {
					if strings.Contains(err.Error(), "UNIQUE constraint failed") {
						s.logger.Info("Initial CIDR block already added by another thread", "network", req.Network, "cidr", req.Ipv4Config.InitialPodCidr)
					} else {
						s.logger.Error(err, "failed to add initial cidr block", "network", req.Network, "cidr", req.Ipv4Config.InitialPodCidr)
						return nil, fmt.Errorf("failed to add initial cidr block %s for network %s: %w", req.Ipv4Config.InitialPodCidr, req.Network, err)
					}
				}
			}
		}

		var ip, cidr string
		var lastErr error
		timeout := s.busyTimeout
		if timeout == 0 {
			timeout = store.DefaultBusyTimeout
		}
		// The total timeout is set to align with the SQLite busy_timeout configured in the DSN.
		// PollUntilContextTimeout creates a derived context with this timeout, but also respects
		// the parent gRPC context (ctx) cancellation.
		// TODO: Measure the store allocation query time and update the interval appropriately.
		err := wait.PollUntilContextTimeout(ctx, 50*time.Millisecond, timeout, true, func(ctx context.Context) (bool, error) {
			ip, cidr, lastErr = s.store.AllocateIPv4(ctx, req.Network, req.Ipv4Config.InterfaceName, req.Ipv4Config.ContainerId)
			if lastErr == nil {
				return true, nil // Success
			}
			if errors.Is(lastErr, store.ErrNoAvailableIPs) {
				return true, lastErr // Stop immediately on non-retryable error
			}
			if ctx.Err() != nil {
				return true, ctx.Err() // Stop immediately if context is done (cancelled or timed out). Prevents misleading "Retrying" logs.
			}
			s.logger.V(4).Info("Retrying AllocateIPv4 due to transient error", "err", lastErr, "network", req.Network)
			return false, nil // Retry
		})

		if err != nil {
			if (errors.Is(err, wait.ErrWaitTimeout) || errors.Is(err, context.DeadlineExceeded)) && lastErr != nil {
				err = lastErr // Use last error if timed out
			}
			s.logger.Error(err, "failed to allocate ipv4", "network", req.Network, "podName", req.PodName, "podNamespace", req.PodNamespace)
			return nil, fmt.Errorf("failed to allocate ipv4 for pod %s/%s: %w", req.PodNamespace, req.PodName, err)
		}
		ipv4Alloc = &adaptiveipam.PodIP{
			IpAddress: ip,
			Cidr:      cidr,
		}
	}

	if req.Ipv6Config != nil {
		// TODO: add ipv6 allocation
	}

	return &adaptiveipam.AllocatePodIPResponse{
		Ipv4: ipv4Alloc,
	}, nil
}

func (s *adaptiveIpamServer) DeallocatePodIP(ctx context.Context, req *adaptiveipam.DeallocatePodIPRequest) (*adaptiveipam.DeallocatePodIPResponse, error) {
	s.logger.Info("DeallocatePodIP request received",
		"network", req.Network,
		"containerID", req.ContainerId,
		"interfaceName", req.InterfaceName,
		"podName", req.PodName,
		"podNamespace", req.PodNamespace)

	count, err := s.store.ReleaseIPByOwner(ctx, req.Network, req.ContainerId, req.InterfaceName, s.releaseCooldown)
	if err != nil {
		s.logger.Error(err, "failed to deallocate ips", "network", req.Network, "podName", req.PodName, "podNamespace", req.PodNamespace)
		return nil, fmt.Errorf("failed to deallocate ips for pod %s/%s: %w", req.PodNamespace, req.PodName, err)
	}

	if count == 0 {
		s.logger.Info("No IP addresses were released (likely already deallocated or didn't exist)", "network", req.Network, "podName", req.PodName, "podNamespace", req.PodNamespace)
	} else {
		s.logger.Info("Successfully deallocated ips", "network", req.Network, "podName", req.PodName, "podNamespace", req.PodNamespace, "count", count)
	}

	return &adaptiveipam.DeallocatePodIPResponse{}, nil
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

	s.grpcServer = grpc.NewServer()
	adaptiveipam.RegisterAdaptiveIpamServer(s.grpcServer, s)

	s.logger.Info("gRPC server is listening", "socket", sockPath)
	return s.grpcServer.Serve(listener)
}

func (s *adaptiveIpamServer) stop() {
	if s.grpcServer != nil {
		s.logger.Info("Stopping gRPC server gracefully")
		s.grpcServer.GracefulStop()
	}
}
