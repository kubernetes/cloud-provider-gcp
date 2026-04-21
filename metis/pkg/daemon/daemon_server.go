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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/metis/api/adaptiveipam/v1"
	"k8s.io/metis/pkg"
	"k8s.io/metis/pkg/store"
)

// TODO: Measure the store allocation query time and update the interval appropriately.
const defaultPollInterval = 50 * time.Millisecond

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
		err := status.Errorf(codes.InvalidArgument, "both ipv4_config and ipv6_config are missing for pod %s/%s", req.PodNamespace, req.PodName)
		s.logger.Error(err, "AllocatePodIP validation failed", "podName", req.PodName, "podNamespace", req.PodNamespace)
		return nil, err
	}

	var ipv4Alloc *adaptiveipam.PodIP
	var err error
	if req.Ipv4Config != nil {
		ipv4Alloc, err = s.allocateIP(ctx, req.Network, req.PodName, req.PodNamespace, req.Ipv4Config.InitialPodCidr, req.Ipv4Config.InterfaceName, req.Ipv4Config.ContainerId, s.store.AllocateIPv4, "4")
		if err != nil {
			return nil, err
		}
	}

	var ipv6Alloc *adaptiveipam.PodIP
	if req.Ipv6Config != nil {
		ipv6Alloc, err = s.allocateIP(ctx, req.Network, req.PodName, req.PodNamespace, req.Ipv6Config.InitialPodCidr, req.Ipv6Config.InterfaceName, req.Ipv6Config.ContainerId, s.store.AllocateIPv6, "6")
		if err != nil {
			return nil, err
		}
	}

	return &adaptiveipam.AllocatePodIPResponse{
		Ipv4: ipv4Alloc,
		Ipv6: ipv6Alloc,
	}, nil
}

func (s *adaptiveIpamServer) allocateIP(ctx context.Context, network string, podName string, podNamespace string, initialPodCidr string, interfaceName string, containerId string, allocateFunc func(context.Context, string, string, string) (string, string, error), ipVersion string) (*adaptiveipam.PodIP, error) {
	if err := s.MaybeAddInitialPodCidr(ctx, network, initialPodCidr); err != nil {
		return nil, err
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
	err := wait.PollUntilContextTimeout(ctx, defaultPollInterval, timeout, true, func(ctx context.Context) (bool, error) {
		ip, cidr, lastErr = allocateFunc(ctx, network, interfaceName, containerId)
		if lastErr == nil {
			return true, nil // Success
		}
		if errors.Is(lastErr, store.ErrNoAvailableIPs) {
			return true, lastErr // Stop immediately on non-retryable error
		}
		if ctx.Err() != nil {
			return true, ctx.Err() // Stop immediately if context is done
		}
		s.logger.V(4).Info(fmt.Sprintf("Retrying AllocateIP%s due to transient error", ipVersion), "err", lastErr, "network", network)
		return false, nil // Retry
	})

	if err != nil {
		if (errors.Is(err, wait.ErrWaitTimeout) || errors.Is(err, context.DeadlineExceeded)) && lastErr != nil {
			err = lastErr // Use last error if timed out
		}
		s.logger.Error(err, fmt.Sprintf("failed to allocate ipv%s", ipVersion), "network", network, "podName", podName, "podNamespace", podNamespace)

		if errors.Is(err, store.ErrNoAvailableIPs) {
			return nil, status.Errorf(codes.ResourceExhausted, "failed to allocate ipv%s for pod %s/%s: %v", ipVersion, podNamespace, podName, err)
		}
		// TODO: Refine status code to return a more specific code based on the error type instead of a fallback Unavailable.
		return nil, status.Errorf(codes.Unavailable, "failed to allocate ipv%s for pod %s/%s: %v", ipVersion, podNamespace, podName, err)
	}

	return &adaptiveipam.PodIP{
		IpAddress: ip,
		Cidr:      cidr,
	}, nil
}

func (s *adaptiveIpamServer) MaybeAddInitialPodCidr(ctx context.Context, network string, initialPodCidr string) error {
	if initialPodCidr == "" {
		return nil
	}

	exists, err := s.store.GetCIDRBlockByCIDR(ctx, initialPodCidr)
	if err != nil {
		s.logger.Error(err, "failed to check if initial cidr block exists", "network", network, "cidr", initialPodCidr)
		return status.Errorf(codes.Unavailable, "failed to check if initial cidr block %s exists for network %s: %v", initialPodCidr, network, err)
	}
	if !exists {
		if err := s.store.AddCIDR(ctx, network, initialPodCidr); err != nil {
			if errors.Is(err, store.ErrCidrAlreadyExists) {
				s.logger.Info("Initial CIDR block already added by another thread", "network", network, "cidr", initialPodCidr)
			} else {
				s.logger.Error(err, "failed to add initial cidr block", "network", network, "cidr", initialPodCidr)
				return status.Errorf(codes.Unavailable, "failed to add initial cidr block %s for network %s: %v", initialPodCidr, network, err)
			}
		}
	}
	return nil
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
		return nil, status.Errorf(codes.Unavailable, "failed to deallocate ips for pod %s/%s: %v", req.PodNamespace, req.PodName, err)
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
