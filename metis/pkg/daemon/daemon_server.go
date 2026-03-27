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

	"google.golang.org/grpc"
	"k8s.io/klog/v2"
	"k8s.io/metis/api/adaptiveipam/v1"
	"k8s.io/metis/pkg"
	"k8s.io/metis/pkg/dal"
)

type adaptiveIpamServer struct {
	adaptiveipam.UnimplementedAdaptiveIpamServer
	dal      *dal.DataAccess
	sockPath string
}

func (s *adaptiveIpamServer) AllocatePodIP(ctx context.Context, req *adaptiveipam.AllocatePodIPRequest) (*adaptiveipam.AllocatePodIPResponse, error) {
	klog.InfoS("AllocatePodIP request received",
		"network", req.Network,
		"podName", req.PodName,
		"podNamespace", req.PodNamespace,
		"ipv4Config", fmt.Sprintf("%+v", req.Ipv4Config),
		"ipv6Config", fmt.Sprintf("%+v", req.Ipv6Config))

	if req.Ipv4Config == nil && req.Ipv6Config == nil {
		err := fmt.Errorf("both ipv4_config and ipv6_config are missing for pod %s/%s", req.PodNamespace, req.PodName)
		klog.ErrorS(err, "AllocatePodIP validation failed", "podName", req.PodName, "podNamespace", req.PodNamespace)
		return nil, err
	}

	var ipv4Alloc *adaptiveipam.PodIP
	if req.Ipv4Config != nil {
		if req.Ipv4Config.InitialPodCidr != "" {
			if err := s.dal.AddCIDR(req.Network, req.Ipv4Config.InitialPodCidr); err != nil {
				klog.ErrorS(err, "failed to add initial cidr block", "network", req.Network, "cidr", req.Ipv4Config.InitialPodCidr)
				return nil, fmt.Errorf("failed to add initial cidr block %s for network %s: %w", req.Ipv4Config.InitialPodCidr, req.Network, err)
			}
		}

		ip, cidr, err := s.dal.AllocateIPv4(req.Network, req.Ipv4Config.InterfaceName, req.Ipv4Config.ContainerId, req.PodName, req.PodNamespace)

		if err != nil {
			klog.ErrorS(err, "failed to allocate ipv4", "network", req.Network, "podName", req.PodName, "podNamespace", req.PodNamespace)
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
	klog.InfoS("DeallocatePodIP request received",
		"network", req.Network,
		"containerID", req.ContainerId,
		"interfaceName", req.InterfaceName,
		"podName", req.PodName,
		"podNamespace", req.PodNamespace)

	count, err := s.dal.ReleaseIPsByOwner(req.Network, req.ContainerId, req.InterfaceName, 0)
	if err != nil {
		klog.ErrorS(err, "failed to deallocate ips", "network", req.Network, "podName", req.PodName, "podNamespace", req.PodNamespace)
		return nil, fmt.Errorf("failed to deallocate ips for pod %s/%s: %w", req.PodNamespace, req.PodName, err)
	}

	if count == 0 {
		klog.InfoS("No IP addresses were released (likely already deallocated or didn't exist)", "network", req.Network, "podName", req.PodName, "podNamespace", req.PodNamespace)
	} else {
		klog.InfoS("Successfully deallocated ips", "network", req.Network, "podName", req.PodName, "podNamespace", req.PodNamespace, "count", count)
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

	grpcServer := grpc.NewServer()
	adaptiveipam.RegisterAdaptiveIpamServer(grpcServer, s)

	klog.InfoS("gRPC server is listening", "socket", sockPath)
	return grpcServer.Serve(listener)
}
