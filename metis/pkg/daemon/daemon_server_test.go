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
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"k8s.io/klog/v2"
	"k8s.io/metis/api/adaptiveipam/v1"
	"k8s.io/metis/pkg/store"
)

func TestAdaptiveIpamServer_withGrpcClient(t *testing.T) {
	logger := logr.Discard()
	tempDir := t.TempDir()
	sockPath := filepath.Join(tempDir, "metis_test_client_integration.sock")
	dbPath := filepath.Join(tempDir, "metis_client_integration.sqlite")

	s, err := store.NewStore(context.Background(), logger, dbPath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer s.Close()

	server := &adaptiveIpamServer{store: s, sockPath: sockPath}

	// 1. Start server in background
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.start()
	}()

	// Wait for socket to appear
	time.Sleep(100 * time.Millisecond)

	// 2. Dial using gRPC client
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, sockPath, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
		return net.Dial("unix", addr)
	}))
	if err != nil {
		t.Fatalf("Failed to dial UDS %s: %v", sockPath, err)
	}
	defer conn.Close()

	client := adaptiveipam.NewAdaptiveIpamClient(conn)

	// 3. Prepare data and call
	network := "integration-network"
	cidr := "10.0.1.0/24"
	req := &adaptiveipam.AllocatePodIPRequest{
		Network:      network,
		PodName:      "test-pod",
		PodNamespace: "default",
		Ipv4Config: &adaptiveipam.IPConfig{
			InterfaceName:  "eth0",
			ContainerId:    "test-container-integration",
			InitialPodCidr: cidr,
		},
	}

	resp, err := client.AllocatePodIP(ctx, req)
	if err != nil {
		t.Fatalf("gRPC Client AllocatePodIP failed: %v", err)
	}

	if resp.Ipv4 == nil || resp.Ipv4.IpAddress == "" {
		t.Errorf("Expected valid IP address from gRPC client, got response: %v", resp)
	}
}

func TestAdaptiveIpamServer_AllocatePodIP(t *testing.T) {
	logger := klog.Background()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "metis_server_test.sqlite")

	storeInstance, err := store.NewStore(context.Background(), logger, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer storeInstance.Close()

	server := &adaptiveIpamServer{store: storeInstance}

	network := "test-network"
	cidr := "10.0.1.0/24"

	req := &adaptiveipam.AllocatePodIPRequest{
		Network:      network,
		PodName:      "test-pod",
		PodNamespace: "default",
		Ipv4Config: &adaptiveipam.IPConfig{
			InterfaceName:  "eth0",
			ContainerId:    "test-container",
			InitialPodCidr: cidr,
		},
	}

	resp, err := server.AllocatePodIP(context.Background(), req)
	if err != nil {
		t.Fatalf("AllocatePodIP failed: %v", err)
	}

	if resp.Ipv4 == nil {
		t.Fatal("Expected Ipv4 allocation, got nil")
	}

	if resp.Ipv4.IpAddress == "" {
		t.Fatal("Expected IP address, got empty string")
	}
}

func TestAdaptiveIpamServer_AllocatePodIP_Concurrency(t *testing.T) {
	logger := klog.Background()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "metis_server_concurrency_test.sqlite")

	storeInstance, err := store.NewStore(context.Background(), logger, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer storeInstance.Close()

	server := &adaptiveIpamServer{store: storeInstance}

	network := "test-network"
	cidr := "10.0.1.0/24"
	cidr6 := "2001:db8::/64"

	numGoroutines := 10
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	ips := make([]string, numGoroutines)
	ips6 := make([]string, numGoroutines)
	errs := make([]error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(index int) {
			defer wg.Done()
			req := &adaptiveipam.AllocatePodIPRequest{
				Network:      network,
				PodName:      fmt.Sprintf("test-pod-%d", index),
				PodNamespace: "default",
				Ipv4Config: &adaptiveipam.IPConfig{
					InterfaceName:  "eth0",
					ContainerId:    fmt.Sprintf("test-container-%d", index/2),
					InitialPodCidr: cidr,
				},
				Ipv6Config: &adaptiveipam.IPConfig{
					InterfaceName:  "eth0",
					ContainerId:    fmt.Sprintf("test-container-%d", index/2),
					InitialPodCidr: cidr6,
				},
			}
			resp, err := server.AllocatePodIP(context.Background(), req)
			if err != nil {
				errs[index] = err
				return
			}
			if resp.Ipv4 != nil {
				ips[index] = resp.Ipv4.IpAddress
			}
			if resp.Ipv6 != nil {
				ips6[index] = resp.Ipv6.IpAddress
			}
		}(i)
	}

	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("Goroutine %d failed: %v", i, err)
		}
	}

	for i, ip := range ips {
		if ip == "" {
			t.Errorf("Goroutine %d returned empty IPv4", i)
		}
	}

	for i, ip := range ips6 {
		if ip == "" {
			t.Errorf("Goroutine %d returned empty IPv6", i)
		}
	}

	for i := 0; i < numGoroutines; i += 2 {
		if ips[i] != "" && ips[i] != ips[i+1] {
			t.Errorf("IPv4 Idempotency check failed for pair %d and %d: expected same IP, got %s and %s", i, i+1, ips[i], ips[i+1])
		}
		if ips6[i] != "" && ips6[i] != ips6[i+1] {
			t.Errorf("IPv6 Idempotency check failed for pair %d and %d: expected same IP, got %s and %s", i, i+1, ips6[i], ips6[i+1])
		}
	}

	uniqueIpMap := make(map[string]bool)
	for _, ip := range ips {
		if ip != "" {
			uniqueIpMap[ip] = true
		}
	}
	if len(uniqueIpMap) != numGoroutines/2 {
		t.Errorf("Expected %d unique IPv4s, got %d (ips: %v)", numGoroutines/2, len(uniqueIpMap), ips)
	}

	uniqueIpMap6 := make(map[string]bool)
	for _, ip := range ips6 {
		if ip != "" {
			uniqueIpMap6[ip] = true
		}
	}
	if len(uniqueIpMap6) != numGoroutines/2 {
		t.Errorf("Expected %d unique IPv6s, got %d (ips6: %v)", numGoroutines/2, len(uniqueIpMap6), ips6)
	}
}

func TestAdaptiveIpamServer_DeallocatePodIP(t *testing.T) {
	logger := logr.Discard()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "metis_daemon_test_release.sqlite")

	s, err := store.NewStore(context.Background(), logger, dbPath)
	if err != nil {
		t.Fatalf("NewStore returned unexpected error: %v", err)
	}
	defer s.Close()

	server := &adaptiveIpamServer{store: s, sockPath: "", releaseCooldown: 1 * time.Minute}

	network := "gke-pod-network"
	cidr := "10.0.1.0/24"
	containerID := "test-container-release"
	interfaceName := "eth0"
	podName := "test-pod"
	podNamespace := "default"

	// 1. Allocate first
	reqAlloc := &adaptiveipam.AllocatePodIPRequest{
		Network:      network,
		PodName:      podName,
		PodNamespace: podNamespace,
		Ipv4Config: &adaptiveipam.IPConfig{
			InterfaceName:  interfaceName,
			ContainerId:    containerID,
			InitialPodCidr: cidr,
		},
	}

	allocResp, err := server.AllocatePodIP(context.Background(), reqAlloc)
	if err != nil {
		t.Fatalf("AllocatePodIP failed in deallocate test setup: %v", err)
	}

	if allocResp.Ipv4 == nil || allocResp.Ipv4.IpAddress == "" {
		t.Fatalf("AllocatePodIP response empty")
	}

	// 2. Deallocate
	reqDealloc := &adaptiveipam.DeallocatePodIPRequest{
		Network:       network,
		InterfaceName: interfaceName,
		ContainerId:   containerID,
		PodName:       podName,
		PodNamespace:  podNamespace,
	}

	deallocResp, err := server.DeallocatePodIP(context.Background(), reqDealloc)
	if err != nil {
		t.Fatalf("DeallocatePodIP failed: %v", err)
	}

	if deallocResp == nil {
		t.Errorf("DeallocatePodIP returned nil response")
	}

	// 3. Verify via store
	var isAlloc bool
	err = s.DB().QueryRow("SELECT is_allocated FROM ip_addresses WHERE address = ?", allocResp.Ipv4.IpAddress).Scan(&isAlloc)
	if err != nil {
		t.Fatalf("Failed to query DB for IP status: %v", err)
	}
	if isAlloc {
		t.Errorf("Expected IP to be unallocated")
	}
}

func TestAdaptiveIpamServer_AllocatePodIP_RetryOnDBError(t *testing.T) {
	logger := klog.Background()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "metis_server_retry_test.sqlite")

	storeInstance, err := store.NewStore(context.Background(), logger, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}

	server := &adaptiveIpamServer{store: storeInstance, busyTimeout: 500 * time.Millisecond}

	network := "test-network"
	cidr := "10.0.1.0/24"

	if err := storeInstance.AddCIDR(context.Background(), network, cidr); err != nil {
		t.Fatalf("Failed to add CIDR: %v", err)
	}

	req := &adaptiveipam.AllocatePodIPRequest{
		Network:      network,
		PodName:      "test-pod",
		PodNamespace: "default",
		Ipv4Config: &adaptiveipam.IPConfig{
			InterfaceName: "eth0",
			ContainerId:   "test-container",
		},
	}

	// Close the DB to simulate transient error
	storeInstance.Close()

	startTime := time.Now()
	_, err = server.AllocatePodIP(context.Background(), req)
	duration := time.Since(startTime)

	if err == nil {
		t.Fatal("Expected error after closing DB, got nil")
	}

	// Expect it to have retried, so duration should be at least 300ms
	if duration < 300*time.Millisecond {
		t.Errorf("Expected test to take at least 300ms due to retries, took %v", duration)
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("Expected gRPC status error, got: %v", err)
	}
	if st.Code() != codes.Unavailable {
		t.Errorf("Expected status code Unavailable, got %v", st.Code())
	}
	if !strings.Contains(st.Message(), "database is closed") {
		t.Errorf("Expected error message to contain 'database is closed', got: %v", st.Message())
	}
}

func TestAdaptiveIpamServer_AllocatePodIP_NoRetryOnExhaustion(t *testing.T) {
	logger := klog.Background()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "metis_server_exhaust_test.sqlite")

	storeInstance, err := store.NewStore(context.Background(), logger, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer storeInstance.Close()

	server := &adaptiveIpamServer{store: storeInstance}

	network := "test-network"

	req := &adaptiveipam.AllocatePodIPRequest{
		Network:      network,
		PodName:      "test-pod",
		PodNamespace: "default",
		Ipv4Config: &adaptiveipam.IPConfig{
			InterfaceName: "eth0",
			ContainerId:   "test-container",
		},
	}

	startTime := time.Now()
	_, err = server.AllocatePodIP(context.Background(), req)
	duration := time.Since(startTime)

	if err == nil {
		t.Fatal("Expected error for exhausted store, got nil")
	}

	// Expect it to fail fast, so duration should be small (much less than 100ms backoff)
	if duration >= 100*time.Millisecond {
		t.Errorf("Expected test to fail fast, but took %v", duration)
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("Expected gRPC status error, got: %v", err)
	}
	if st.Code() != codes.ResourceExhausted {
		t.Errorf("Expected status code ResourceExhausted, got %v", st.Code())
	}
	if !strings.Contains(st.Message(), store.ErrNoAvailableIPs.Error()) {
		t.Errorf("Expected status message to contain '%v', got: %s", store.ErrNoAvailableIPs, st.Message())
	}
}

func TestAdaptiveIpamServer_AllocatePodIP_IPv6(t *testing.T) {
	logger := klog.Background()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "metis_server_ipv6_test.sqlite")

	storeInstance, err := store.NewStore(context.Background(), logger, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer storeInstance.Close()

	server := &adaptiveIpamServer{store: storeInstance}

	network := "test-network"
	cidr := "2001:db8::/64"

	req := &adaptiveipam.AllocatePodIPRequest{
		Network:      network,
		PodName:      "test-pod",
		PodNamespace: "default",
		Ipv6Config: &adaptiveipam.IPConfig{
			InterfaceName:  "eth0",
			ContainerId:    "test-container",
			InitialPodCidr: cidr,
		},
	}

	resp, err := server.AllocatePodIP(context.Background(), req)
	if err != nil {
		t.Fatalf("AllocatePodIP failed: %v", err)
	}

	if resp.Ipv6 == nil {
		t.Fatal("Expected Ipv6 allocation, got nil")
	}

	if resp.Ipv6.IpAddress == "" {
		t.Fatal("Expected IP address, got empty string")
	}

	// Verify it's a valid IPv6 address
	ip := net.ParseIP(resp.Ipv6.IpAddress)
	if ip == nil || ip.To4() != nil {
		t.Errorf("Expected valid IPv6 address, got %s", resp.Ipv6.IpAddress)
	}
}

func TestAdaptiveIpamServer_AllocatePodIP_IPv6_Idempotency_Release(t *testing.T) {
	logger := klog.Background()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "metis_server_ipv6_idempotency_test.sqlite")

	storeInstance, err := store.NewStore(context.Background(), logger, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer storeInstance.Close()

	server := &adaptiveIpamServer{store: storeInstance}

	network := "test-network"
	cidr := "2001:db8::/64"
	containerID := "test-container"
	interfaceName := "eth0"
	podName := "test-pod"
	podNamespace := "default"

	req := &adaptiveipam.AllocatePodIPRequest{
		Network:      network,
		PodName:      podName,
		PodNamespace: podNamespace,
		Ipv6Config: &adaptiveipam.IPConfig{
			InterfaceName:  interfaceName,
			ContainerId:    containerID,
			InitialPodCidr: cidr,
		},
	}

	// 1. First allocation
	resp1, err := server.AllocatePodIP(context.Background(), req)
	if err != nil {
		t.Fatalf("First allocation failed: %v", err)
	}
	if resp1.Ipv6 == nil || resp1.Ipv6.IpAddress == "" {
		t.Fatal("Expected Ipv6 allocation, got nil or empty")
	}

	// 2. Second allocation (idempotency)
	resp2, err := server.AllocatePodIP(context.Background(), req)
	if err != nil {
		t.Fatalf("Second allocation failed: %v", err)
	}
	if resp2.Ipv6 == nil || resp2.Ipv6.IpAddress != resp1.Ipv6.IpAddress {
		t.Errorf("Idempotency failed: expected IP %s, got %v", resp1.Ipv6.IpAddress, resp2.Ipv6)
	}

	// 3. Release
	reqDealloc := &adaptiveipam.DeallocatePodIPRequest{
		Network:       network,
		InterfaceName: interfaceName,
		ContainerId:   containerID,
		PodName:       podName,
		PodNamespace:  podNamespace,
	}
	_, err = server.DeallocatePodIP(context.Background(), reqDealloc)
	if err != nil {
		t.Fatalf("DeallocatePodIP failed: %v", err)
	}

	// 4. Re-allocate
	resp3, err := server.AllocatePodIP(context.Background(), req)
	if err != nil {
		t.Fatalf("Re-allocation failed: %v", err)
	}
	// Cooldown is 0 by default in the server struct if not set, so it should reuse the IP immediately.
	if resp3.Ipv6 == nil || resp3.Ipv6.IpAddress != resp1.Ipv6.IpAddress {
		t.Errorf("Expected released IP %s to be reused, got %v", resp1.Ipv6.IpAddress, resp3.Ipv6)
	}
}

func TestAdaptiveIpamServer_AllocatePodIP_DualStack(t *testing.T) {
	logger := klog.Background()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "metis_server_dualstack_test.sqlite")

	storeInstance, err := store.NewStore(context.Background(), logger, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer storeInstance.Close()

	server := &adaptiveIpamServer{store: storeInstance}

	network := "test-network"
	cidr4 := "10.0.1.0/24"
	cidr6 := "2001:db8::/64"

	req := &adaptiveipam.AllocatePodIPRequest{
		Network:      network,
		PodName:      "test-pod",
		PodNamespace: "default",
		Ipv4Config: &adaptiveipam.IPConfig{
			InterfaceName:  "eth0",
			ContainerId:    "test-container",
			InitialPodCidr: cidr4,
		},
		Ipv6Config: &adaptiveipam.IPConfig{
			InterfaceName:  "eth0",
			ContainerId:    "test-container",
			InitialPodCidr: cidr6,
		},
	}

	resp, err := server.AllocatePodIP(context.Background(), req)
	if err != nil {
		t.Fatalf("AllocatePodIP failed: %v", err)
	}

	if resp.Ipv4 == nil || resp.Ipv4.IpAddress == "" {
		t.Fatal("Expected IPv4 allocation, got nil or empty")
	}

	if resp.Ipv6 == nil || resp.Ipv6.IpAddress == "" {
		t.Fatal("Expected IPv6 allocation, got nil or empty")
	}

	// Verify valid IPs
	ip4 := net.ParseIP(resp.Ipv4.IpAddress)
	if ip4 == nil || ip4.To4() == nil {
		t.Errorf("Expected valid IPv4 address, got %s", resp.Ipv4.IpAddress)
	}

	ip6 := net.ParseIP(resp.Ipv6.IpAddress)
	if ip6 == nil || ip6.To4() != nil {
		t.Errorf("Expected valid IPv6 address, got %s", resp.Ipv6.IpAddress)
	}
}
