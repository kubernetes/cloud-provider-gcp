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
	"sync"
	"time"

	networkv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/network/v1"
	"github.com/go-logr/logr"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/metis/api/adaptiveipam/v1"
	"k8s.io/metis/pkg/store"
)

const (
	defaultPollInterval = 50 * time.Millisecond
	// scaleUpWaitTimeout is the maximum time to wait for a dynamic scale-up operation.
	// It must be smaller than the CNI client-side RPC timeout (defaultRPCTimeout = 10s)
	// to allow the server to return a clean error response before the client times out.
	scaleUpWaitTimeout = 9 * time.Second
)

type cniClient struct {
	containerID  string
	network      string
	podName      string
	podNamespace string
}

// IPAMEngine manages the core transport-agnostic business logic for IP address allocation,
// deallocation, and validation against the local SQLite store.
type IPAMEngine struct {
	store           *store.Store
	releaseCooldown time.Duration
	busyTimeout     time.Duration
	logger          logr.Logger
	// requestsMap tracks pending CNI IP allocation requests waiting for a new CIDR
	// to be dynamically allocated. It is organized per-network (outer map key is network name)
	// to optimize lookups and avoid iterating over all waiting clients from other networks.
	// The inner map associates each blocked cniClient to a channel that is closed to wake
	// it up when new IPs become available.
	requestsMap map[string]map[cniClient]chan struct{}
	requestsMu  sync.RWMutex
	monitor     *Monitor
}

// NewIPAMEngine constructs a new IPAMEngine instance.
func NewIPAMEngine(logger logr.Logger, storeInstance *store.Store, releaseCooldown time.Duration, busyTimeout time.Duration, monitor *Monitor) *IPAMEngine {
	return &IPAMEngine{
		store:           storeInstance,
		releaseCooldown: releaseCooldown,
		busyTimeout:     busyTimeout,
		logger:          logger,
		requestsMap:     make(map[string]map[cniClient]chan struct{}),
		monitor:         monitor,
	}
}

// SetMonitor updates the monitor associated with the IPAMEngine.
func (e *IPAMEngine) SetMonitor(m *Monitor) {
	e.requestsMu.Lock()
	defer e.requestsMu.Unlock()
	e.monitor = m
}

// AllocatePodIP allocates IPv4 and/or IPv6 addresses for a pod request.
func (e *IPAMEngine) AllocatePodIP(ctx context.Context, req *adaptiveipam.AllocatePodIPRequest) (*adaptiveipam.AllocatePodIPResponse, error) {
	if req.Network == "" {
		req.Network = networkv1.DefaultPodNetworkName
	}

	e.logger.Info("AllocatePodIP request received",
		"network", req.Network,
		"podName", req.PodName,
		"podNamespace", req.PodNamespace,
		"ipv4Config", fmt.Sprintf("%+v", req.Ipv4Config),
		"ipv6Config", fmt.Sprintf("%+v", req.Ipv6Config))

	if req.Ipv4Config == nil && req.Ipv6Config == nil {
		err := status.Errorf(codes.InvalidArgument, "both ipv4_config and ipv6_config are missing for pod %s/%s", req.PodNamespace, req.PodName)
		e.logger.Error(err, "AllocatePodIP validation failed", "podName", req.PodName, "podNamespace", req.PodNamespace)
		return nil, err
	}

	// Enforce a server-side safety timeout ceiling for the entire allocation attempt.
	// This must be shorter than the client CNI plugin's timeout to ensure the server
	// fails gracefully and returns a structured gRPC error before the client gives up.
	ctx, cancel := context.WithTimeout(ctx, scaleUpWaitTimeout)
	defer cancel()

	var ipv4Alloc *adaptiveipam.PodIP
	var err error
	if req.Ipv4Config != nil {
		if req.Ipv4Config.ContainerId == "" || req.Ipv4Config.InterfaceName == "" {
			return nil, status.Error(codes.InvalidArgument, "container_id and interface_name must not be empty")
		}
		ipv4Alloc, err = e.allocateIP(ctx, req, req.Ipv4Config, store.IPv4)
		if err != nil {
			return nil, err
		}
	}

	var ipv6Alloc *adaptiveipam.PodIP
	if req.Ipv6Config != nil {
		if req.Ipv6Config.ContainerId == "" || req.Ipv6Config.InterfaceName == "" {
			return nil, status.Error(codes.InvalidArgument, "container_id and interface_name must not be empty")
		}
		ipv6Alloc, err = e.allocateIP(ctx, req, req.Ipv6Config, store.IPv6)
		if err != nil {
			return nil, err
		}
	}

	return &adaptiveipam.AllocatePodIPResponse{
		Ipv4: ipv4Alloc,
		Ipv6: ipv6Alloc,
	}, nil
}

func (e *IPAMEngine) allocateIP(ctx context.Context, req *adaptiveipam.AllocatePodIPRequest, config *adaptiveipam.IPConfig, ipFamily store.IPFamily) (*adaptiveipam.PodIP, error) {
	if err := e.maybeAddInitialPodCidr(ctx, req.Network, config.InitialPodCidr); err != nil {
		return nil, err
	}

	timeout := e.busyTimeout
	if timeout == 0 {
		timeout = store.DefaultBusyTimeout
	}

	params := store.AllocateIPParams{
		Network:       req.Network,
		InterfaceName: config.InterfaceName,
		ContainerID:   config.ContainerId,
		IPFamily:      ipFamily,
	}

	// The loop is bounded by the cancellation or timeout of the context ctx.
	for {
		ip, cidr, err := e.allocateIPWithRetry(ctx, params, timeout)
		if err != nil {
			e.logger.Error(err, fmt.Sprintf("failed to allocate %s", ipFamily), "network", req.Network, "podName", req.PodName, "podNamespace", req.PodNamespace)

			capacityAdded, allocErr := e.maybeDynamicAllocation(ctx, req, params, err)
			if allocErr != nil {
				return nil, allocErr
			}
			if capacityAdded {
				continue
			}
			// TODO: Refine status code to return a more specific code based on the error type instead of a fallback Unavailable.
			return nil, status.Errorf(codes.Unavailable, "failed to allocate %s for pod %s/%s: %v", ipFamily, req.PodNamespace, req.PodName, err)
		}

		return &adaptiveipam.PodIP{
			IpAddress: ip,
			Cidr:      cidr,
		}, nil
	}
}

func (e *IPAMEngine) allocateIPWithRetry(ctx context.Context, params store.AllocateIPParams, timeout time.Duration) (string, string, error) {
	var ip, cidr string
	var lastErr error

	// The total timeout is set to align with the SQLite busy_timeout configured in the DSN.
	// PollUntilContextTimeout creates a derived context with this timeout, but also respects
	// the parent gRPC context (ctx) cancellation.
	err := wait.PollUntilContextTimeout(ctx, defaultPollInterval, timeout, true, func(ctx context.Context) (bool, error) {
		ip, cidr, lastErr = e.store.AllocateIP(ctx, params)
		if lastErr == nil {
			return true, nil // Success
		}
		if errors.Is(lastErr, store.ErrNoAvailableIPs) {
			return true, lastErr // Stop immediately on non-retryable error
		}
		if ctx.Err() != nil {
			return true, ctx.Err() // Stop immediately if context is done
		}
		e.logger.V(4).Info(fmt.Sprintf("Retrying %s allocation due to transient error", params.IPFamily), "err", lastErr, "network", params.Network)
		return false, nil // Retry
	})

	if err != nil {
		if (errors.Is(err, wait.ErrWaitTimeout) || errors.Is(err, context.DeadlineExceeded)) && lastErr != nil {
			err = lastErr // Use last error if timed out
		}
		return "", "", err
	}

	return ip, cidr, nil
}

func (e *IPAMEngine) handleDynamicAllocation(ctx context.Context, req *adaptiveipam.AllocatePodIPRequest) error {
	clientKey := cniClient{
		containerID:  req.Ipv4Config.ContainerId,
		network:      req.Network,
		podName:      req.PodName,
		podNamespace: req.PodNamespace,
	}

	if e.monitor == nil {
		e.logger.V(2).Info("No monitor available, failing fast on exhaustion", "network", req.Network)
		return fmt.Errorf("failed to allocate ipv4 for pod %s/%s: %w", req.PodNamespace, req.PodName, store.ErrNoAvailableIPs)
	}

	ch, ok := e.getOrCreatePendingRequest(clientKey, req.Network)

	if !ok {
		e.logger.Info("Local store IP exhaustion detected, requesting scale up", "network", req.Network, "podName", req.PodName, "podNamespace", req.PodNamespace)
		// Enqueue the request to trigger the controller sync for dynamic allocation.
		e.monitor.enqueue()
	} else {
		e.logger.Info("Dynamic allocation request already pending, waiting on existing request", "network", req.Network, "podName", req.PodName, "podNamespace", req.PodNamespace)
	}

	select {
	case <-ctx.Done():
		e.removePendingRequest(clientKey, req.Network)
		e.logger.Error(ctx.Err(), "Dynamic allocation wait timed out or cancelled", "network", req.Network, "podName", req.PodName, "podNamespace", req.PodNamespace)
		return fmt.Errorf("failed to allocate ipv4 for pod %s/%s (timed out): %w", req.PodNamespace, req.PodName, store.ErrNoAvailableIPs)
	case <-ch:
		e.logger.Info("Woken up by CIDR watcher, retrying local allocation", "network", req.Network, "podName", req.PodName, "podNamespace", req.PodNamespace)
		return nil
	}
}

func (e *IPAMEngine) maybeDynamicAllocation(ctx context.Context, req *adaptiveipam.AllocatePodIPRequest, params store.AllocateIPParams, err error) (bool, error) {
	if params.IPFamily != store.IPv4 || !errors.Is(err, store.ErrNoAvailableIPs) {
		return false, nil
	}

	undrained, undrainErr := e.store.UndrainOneCIDRBlock(ctx, req.Network, store.IPv4)
	if undrainErr == nil && undrained {
		e.logger.Info("Successfully undrained one CIDR block, retrying local allocation", "network", req.Network, "podName", req.PodName, "podNamespace", req.PodNamespace)
		return true, nil
	}

	if err := e.handleDynamicAllocation(ctx, req); err != nil {
		return false, status.Errorf(codes.ResourceExhausted, "failed to allocate ipv4 for pod %s/%s: %v", req.PodNamespace, req.PodName, err)
	}

	return true, nil
}

func (e *IPAMEngine) maybeAddInitialPodCidr(ctx context.Context, network string, initialPodCidr string) error {
	if initialPodCidr == "" {
		return nil
	}
	// TODO: save a bool flag about whether we added the initial CIDR to the store to avoid calling store everytime to check if initial cidr is added
	_, exists, err := e.store.GetCIDRBlock(ctx, initialPodCidr, network)

	if err != nil {
		e.logger.Error(err, "failed to check if initial cidr block exists", "network", network, "cidr", initialPodCidr)
		return status.Errorf(codes.Unavailable, "failed to check if initial cidr block %s exists for network %s: %v", initialPodCidr, network, err)
	}
	if !exists {
		if err := e.store.AddCIDR(ctx, network, initialPodCidr); err != nil {
			if errors.Is(err, store.ErrCidrAlreadyExists) {
				e.logger.V(4).Info("Initial CIDR block already added by another thread", "network", network, "cidr", initialPodCidr)
			} else {
				e.logger.Error(err, "failed to add initial cidr block", "network", network, "cidr", initialPodCidr)
				return status.Errorf(codes.Unavailable, "failed to add initial cidr block %s for network %s: %v", initialPodCidr, network, err)
			}
		}
	}
	return nil
}

// DeallocatePodIP releases IP reservations owned by a container and interface.
func (e *IPAMEngine) DeallocatePodIP(ctx context.Context, req *adaptiveipam.DeallocatePodIPRequest) (*adaptiveipam.DeallocatePodIPResponse, error) {
	if req.Network == "" {
		req.Network = networkv1.DefaultPodNetworkName
	}

	e.logger.Info("DeallocatePodIP request received",
		"network", req.Network,
		"containerID", req.ContainerId,
		"interfaceName", req.InterfaceName,
		"podName", req.PodName,
		"podNamespace", req.PodNamespace)

	if req.ContainerId == "" || req.InterfaceName == "" {
		return nil, status.Error(codes.InvalidArgument, "container_id and interface_name must not be empty")
	}

	releasedIPs, err := e.store.ReleaseIPByOwner(ctx, req.Network, req.ContainerId, req.InterfaceName, e.releaseCooldown)
	if err != nil {
		e.logger.Error(err, "failed to deallocate ips", "network", req.Network, "podName", req.PodName, "podNamespace", req.PodNamespace)
		return nil, status.Errorf(codes.Unavailable, "failed to deallocate ips for pod %s/%s: %v", req.PodNamespace, req.PodName, err)
	}

	if len(releasedIPs) == 0 {
		e.logger.V(4).Info("No IP addresses were released (likely already deallocated or didn't exist)", "network", req.Network, "podName", req.PodName, "podNamespace", req.PodNamespace)
	} else {
		e.logger.Info("Successfully deallocated ips",
			"network", req.Network,
			"podName", req.PodName,
			"podNamespace", req.PodNamespace,
			"releasedIPs", releasedIPs,
			"count", len(releasedIPs))
	}

	return &adaptiveipam.DeallocatePodIPResponse{}, nil
}

func (e *IPAMEngine) getPendingRequestsCount(network string) int {
	e.requestsMu.RLock()
	defer e.requestsMu.RUnlock()

	return len(e.requestsMap[network])
}

func (e *IPAMEngine) onCIDRAdded(network string, availableIPs int) {
	e.requestsMu.Lock()
	defer e.requestsMu.Unlock()

	e.logger.Info("CIDR added, checking for waiting CNI requests to wake up", "network", network, "availableIPs", availableIPs)

	netMap := e.requestsMap[network]
	if len(netMap) == 0 {
		return
	}

	var awakenedClients []string
	count := 0
	for client, ch := range netMap {
		close(ch)
		delete(netMap, client)
		clientDetail := fmt.Sprintf("%s/%s", client.podNamespace, client.podName)
		if client.containerID != "" {
			clientDetail = fmt.Sprintf("%s/%s (containerID: %s)", client.podNamespace, client.podName, client.containerID)
		}
		awakenedClients = append(awakenedClients, clientDetail)
		count++
		if count >= availableIPs {
			break
		}
	}

	if len(netMap) == 0 {
		delete(e.requestsMap, network)
	}

	if count > 0 {
		e.logger.Info("Successfully woke up waiting CNI requests", "network", network, "count", count)
		e.logger.V(4).Info("Awakened CNI client requests details", "network", network, "clients", awakenedClients)
	}
}

// CheckPodIP verifies if an IP allocation exists for a pod.
func (e *IPAMEngine) CheckPodIP(ctx context.Context, req *adaptiveipam.CheckPodIPRequest) (*adaptiveipam.CheckPodIPResponse, error) {
	if req.Network == "" {
		req.Network = networkv1.DefaultPodNetworkName
	}

	e.logger.Info("CheckPodIP request received",
		"network", req.Network,
		"containerID", req.ContainerId,
		"interfaceName", req.InterfaceName,
		"podName", req.PodName,
		"podNamespace", req.PodNamespace)

	err := e.store.CheckAllocation(ctx, req.Network, req.ContainerId, req.InterfaceName)
	if err != nil {
		e.logger.Error(err, "CheckPodIP failed", "network", req.Network, "containerID", req.ContainerId, "podName", req.PodName, "podNamespace", req.PodNamespace)
		return nil, status.Errorf(codes.NotFound, "allocation check failed: %v", err)
	}

	e.logger.Info("CheckPodIP succeeded", "network", req.Network, "containerID", req.ContainerId, "podName", req.PodName, "podNamespace", req.PodNamespace)
	return &adaptiveipam.CheckPodIPResponse{}, nil
}

// getOrCreatePendingRequest retrieves the wakeup channel for a pending CNI client request,
// creating it if it does not already exist. It returns the channel and a boolean indicating
// whether the channel already existed.
func (e *IPAMEngine) getOrCreatePendingRequest(clientKey cniClient, network string) (chan struct{}, bool) {
	e.requestsMu.Lock()
	defer e.requestsMu.Unlock()

	netMap, netOk := e.requestsMap[network]
	if !netOk {
		netMap = make(map[cniClient]chan struct{})
		e.requestsMap[network] = netMap
	}
	ch, ok := netMap[clientKey]
	if !ok {
		ch = make(chan struct{})
		netMap[clientKey] = ch
	}
	return ch, ok
}

// removePendingRequest removes a CNI client request from the pending map once it has completed
// or timed out.
func (e *IPAMEngine) removePendingRequest(clientKey cniClient, network string) {
	e.requestsMu.Lock()
	defer e.requestsMu.Unlock()

	if netMap, ok := e.requestsMap[network]; ok {
		delete(netMap, clientKey)
		if len(netMap) == 0 {
			delete(e.requestsMap, network)
		}
	}
}
