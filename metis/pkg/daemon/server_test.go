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

	nncv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/nodenetworkconfig/v1"
	nncfake "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/clientset/versioned/fake"
	"github.com/go-logr/logr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	server := newAdaptiveIpamServer(logger, storeInstance, "", 0, 0)

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

	server := newAdaptiveIpamServer(logger, storeInstance, "", 0, 0)

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

	server := newAdaptiveIpamServer(logger, s, "", 1*time.Minute, 0)

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

	server := newAdaptiveIpamServer(logger, storeInstance, "", 0, 500*time.Millisecond)

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
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err = server.AllocatePodIP(ctx, req)
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

	server := newAdaptiveIpamServer(logger, storeInstance, "", 0, 0)

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
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	_, err = server.AllocatePodIP(ctx, req)
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

func TestAdaptiveIpamServer_AllocatePodIP_DynamicAllocation(t *testing.T) {
	logger := klog.Background()

	tests := []struct {
		name         string
		cancelCtx    bool
		action       func(ctx context.Context, server *adaptiveIpamServer, storeInstance *store.Store, network string) error
		wantErr      bool
		errSubstring string
		checkResp    func(t *testing.T, resp *adaptiveipam.AllocatePodIPResponse)
	}{
		{
			name:      "Success (Regular Path)",
			cancelCtx: false,
			action: func(ctx context.Context, server *adaptiveIpamServer, storeInstance *store.Store, network string) error {
				if err := storeInstance.AddCIDR(ctx, network, "10.0.1.0/24"); err != nil {
					return err
				}
				server.onCIDRAdded(network, 256)
				return nil
			},
			wantErr: false,
			checkResp: func(t *testing.T, resp *adaptiveipam.AllocatePodIPResponse) {
				if resp.Ipv4 == nil || resp.Ipv4.IpAddress == "" {
					t.Errorf("Expected valid IP address, got response: %v", resp)
				}
			},
		},
		{
			name:         "Context Cancelled Path",
			cancelCtx:    true,
			action:       func(ctx context.Context, server *adaptiveIpamServer, storeInstance *store.Store, network string) error { return nil },
			wantErr:      true,
			errSubstring: "timed out",
			checkResp: func(t *testing.T, resp *adaptiveipam.AllocatePodIPResponse) {
				if resp != nil {
					t.Errorf("Expected nil response on cancellation, got: %v", resp)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tempDir := t.TempDir()
			dbPath := filepath.Join(tempDir, "metis_server_dynamic_test.sqlite")

			storeInstance, err := store.NewStore(context.Background(), logger, dbPath)
			if err != nil {
				t.Fatalf("Failed to create store: %v", err)
			}
			defer storeInstance.Close()

			server := newAdaptiveIpamServer(logger, storeInstance, "", 0, 0)

			nodeName := "test-node"
			mockNNC := &nncv1.NodeNetworkConfig{
				ObjectMeta: metav1.ObjectMeta{Name: nodeName},
			}
			nncClient := nncfake.NewSimpleClientset(mockNNC)

			monitorInstance := NewMonitor(MonitorConfig{
				Logger:          logger,
				NNCClient:       nncClient,
				Store:           storeInstance,
				NodeName:        nodeName,
				MonitorInterval: 1 * time.Second,
			})
			server.monitor = monitorInstance

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

			var ctx context.Context
			var cancel context.CancelFunc

			if tc.cancelCtx {
				ctx, cancel = context.WithCancel(context.Background())
			} else {
				ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
			}
			defer cancel()

			var wg sync.WaitGroup
			wg.Add(1)
			var resp *adaptiveipam.AllocatePodIPResponse
			var allocErr error

			go func() {
				defer wg.Done()
				resp, allocErr = server.AllocatePodIP(ctx, req)
			}()

			// Wait for the request to be enqueued and waiting in map
			time.Sleep(100 * time.Millisecond)

			// Verify that there is a pending request in map
			server.requestsMu.RLock()
			mapLen := len(server.requestsMap)
			server.requestsMu.RUnlock()
			if mapLen != 1 {
				t.Errorf("Expected 1 pending request in requestsMap, got %d", mapLen)
			}

			if tc.cancelCtx {
				cancel()
			} else {
				if err := tc.action(ctx, server, storeInstance, network); err != nil {
					t.Fatalf("Test case action failed: %v", err)
				}
			}

			// Wait for the goroutine to finish
			wg.Wait()

			if tc.wantErr {
				if allocErr == nil {
					t.Fatal("Expected AllocatePodIP to fail, but it succeeded")
				}
				if tc.errSubstring != "" && !strings.Contains(allocErr.Error(), tc.errSubstring) {
					t.Errorf("Expected error to contain %q, got: %v", tc.errSubstring, allocErr)
				}
			} else {
				if allocErr != nil {
					t.Fatalf("AllocatePodIP failed: %v", allocErr)
				}
			}

			if tc.checkResp != nil {
				tc.checkResp(t, resp)
			}

			// Verify requestsMap is empty
			server.requestsMu.RLock()
			finalMapLen := len(server.requestsMap)
			server.requestsMu.RUnlock()
			if finalMapLen != 0 {
				t.Errorf("Expected requestsMap to be empty (0 entries), but got %d entries", finalMapLen)
			}
		})
	}
}

func TestAdaptiveIpamServer_AllocatePodIP_DynamicAllocation_MultipleRequests(t *testing.T) {
	logger := klog.Background()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "metis_server_dynamic_multi_test.sqlite")

	storeInstance, err := store.NewStore(context.Background(), logger, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer storeInstance.Close()

	server := newAdaptiveIpamServer(logger, storeInstance, "", 0, 10*time.Second)

	nodeName := "test-node"
	mockNNC := &nncv1.NodeNetworkConfig{
		ObjectMeta: metav1.ObjectMeta{Name: nodeName},
	}
	nncClient := nncfake.NewSimpleClientset(mockNNC)

	monitorInstance := NewMonitor(MonitorConfig{
		Logger:          logger,
		NNCClient:       nncClient,
		Store:           storeInstance,
		NodeName:        nodeName,
		MonitorInterval: 1 * time.Second,
	})
	server.monitor = monitorInstance

	network := "test-network"
	numRequests := 10

	var wg sync.WaitGroup
	wg.Add(numRequests)

	resps := make([]*adaptiveipam.AllocatePodIPResponse, numRequests)
	errs := make([]error, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(index int) {
			defer wg.Done()
			req := &adaptiveipam.AllocatePodIPRequest{
				Network:      network,
				PodName:      fmt.Sprintf("test-pod-%d", index),
				PodNamespace: "default",
				Ipv4Config: &adaptiveipam.IPConfig{
					InterfaceName:  "eth0",
					ContainerId:    fmt.Sprintf("test-container-%d", index),
					InitialPodCidr: "10.0.0.0/29",
				},
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			resps[index], errs[index] = server.AllocatePodIP(ctx, req)
		}(i)
	}

	// Wait for all requests to be enqueued and waiting in map
	time.Sleep(200 * time.Millisecond)

	// Verify that there are pending requests in map (5 should be pending as 5 succeeded from initial block)
	if server.getPendingRequestsCount(network) != 5 {
		t.Errorf("Expected 5 pending requests, got %d", server.getPendingRequestsCount(network))
	}

	// Now simulate the controller adding a CIDR (/29 has 8 IPs)
	err = storeInstance.AddCIDR(context.Background(), network, "10.0.1.0/29")
	if err != nil {
		t.Fatalf("Failed to add CIDR: %v", err)
	}

	// Wake up first 8 requests (matching the block size)
	server.onCIDRAdded(network, 8)

	// Wait for all to finish
	wg.Wait()

	successCount := 0
	for i := 0; i < numRequests; i++ {
		if errs[i] == nil && resps[i] != nil && resps[i].Ipv4 != nil && resps[i].Ipv4.IpAddress != "" {
			successCount++
		} else if errs[i] != nil {
			t.Errorf("Request %d failed: %v", i, errs[i])
		}
	}

	if successCount != numRequests {
		t.Errorf("Expected %d successful allocations at the end, got %d", numRequests, successCount)
	}

	if server.getPendingRequestsCount(network) != 0 {
		t.Errorf("Expected 0 pending requests left, got %d", server.getPendingRequestsCount(network))
	}
}

func TestAdaptiveIpamServer_CheckPodIP(t *testing.T) {
	logger := logr.Discard()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "metis_daemon_check_test.sqlite")

	s, err := store.NewStore(context.Background(), logger, dbPath)
	if err != nil {
		t.Fatalf("NewStore failed: %v", err)
	}
	defer s.Close()

	server := &adaptiveIpamServer{store: s}

	network := "test-network"
	cidr := "10.0.1.0/24"
	containerID := "test-container"
	interfaceName := "eth0"
	podName := "test-pod"
	podNamespace := "default"

	if err := s.AddCIDR(context.Background(), network, cidr); err != nil {
		t.Fatalf("Failed to add CIDR: %v", err)
	}

	// 1. Check before allocation (should return NotFound)
	req := &adaptiveipam.CheckPodIPRequest{
		Network:       network,
		InterfaceName: interfaceName,
		ContainerId:   containerID,
		PodName:       podName,
		PodNamespace:  podNamespace,
	}

	_, err = server.CheckPodIP(context.Background(), req)
	if err == nil {
		t.Fatal("Expected error for non-existent allocation, got nil")
	}
	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("Expected gRPC status error, got: %v", err)
	}
	if st.Code() != codes.NotFound {
		t.Errorf("Expected status code NotFound, got %v", st.Code())
	}

	// 2. Allocate IP
	reqAlloc := &adaptiveipam.AllocatePodIPRequest{
		Network:      network,
		PodName:      podName,
		PodNamespace: podNamespace,
		Ipv4Config: &adaptiveipam.IPConfig{
			InterfaceName: interfaceName,
			ContainerId:   containerID,
		},
	}
	_, err = server.AllocatePodIP(context.Background(), reqAlloc)
	if err != nil {
		t.Fatalf("AllocatePodIP failed: %v", err)
	}

	// 3. Check after allocation (should succeed)
	resp, err := server.CheckPodIP(context.Background(), req)
	if err != nil {
		t.Fatalf("CheckPodIP failed after allocation: %v", err)
	}
	if resp == nil {
		t.Fatal("Expected non-nil response")
	}
}

func TestAdaptiveIpamServer_AllocatePodIP_Validation(t *testing.T) {
	server := &adaptiveIpamServer{}

	tests := []struct {
		name          string
		containerID   string
		interfaceName string
		testIPv6      bool
	}{
		{"ipv4 empty both", "", "", false},
		{"ipv4 empty container", "", "eth0", false},
		{"ipv4 empty interface", "cont1", "", false},
		{"ipv6 empty both", "", "", true},
		{"ipv6 empty container", "", "eth0", true},
		{"ipv6 empty interface", "cont1", "", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := &adaptiveipam.AllocatePodIPRequest{
				Network:      "test-network",
				PodName:      "test-pod",
				PodNamespace: "default",
			}
			if tc.testIPv6 {
				req.Ipv6Config = &adaptiveipam.IPConfig{
					InterfaceName: tc.interfaceName,
					ContainerId:   tc.containerID,
				}
			} else {
				req.Ipv4Config = &adaptiveipam.IPConfig{
					InterfaceName: tc.interfaceName,
					ContainerId:   tc.containerID,
				}
			}

			_, err := server.AllocatePodIP(context.Background(), req)
			if err == nil {
				t.Fatal("Expected error, got nil")
			}
			st, ok := status.FromError(err)
			if !ok {
				t.Fatalf("Expected gRPC status error, got: %v", err)
			}
			if st.Code() != codes.InvalidArgument {
				t.Errorf("Expected status code InvalidArgument, got %v", st.Code())
			}
		})
	}
}

func TestAdaptiveIpamServer_DeallocatePodIP_Validation(t *testing.T) {
	server := &adaptiveIpamServer{}

	tests := []struct {
		name          string
		containerID   string
		interfaceName string
	}{
		{"empty both", "", ""},
		{"empty container", "", "eth0"},
		{"empty interface", "cont1", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := &adaptiveipam.DeallocatePodIPRequest{
				Network:       "test-network",
				InterfaceName: tc.interfaceName,
				ContainerId:   tc.containerID,
				PodName:       "test-pod",
				PodNamespace:  "default",
			}

			_, err := server.DeallocatePodIP(context.Background(), req)
			if err == nil {
				t.Fatal("Expected error, got nil")
			}
			st, ok := status.FromError(err)
			if !ok {
				t.Fatalf("Expected gRPC status error, got: %v", err)
			}
			if st.Code() != codes.InvalidArgument {
				t.Errorf("Expected status code InvalidArgument, got %v", st.Code())
			}
		})
	}
}

