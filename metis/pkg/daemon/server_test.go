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
	"net"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"k8s.io/metis/api/adaptiveipam/v1"
	"k8s.io/metis/pkg/metrics"
	"k8s.io/metis/pkg/store"
)

type testMetricsRecorder struct {
	metrics.MetricsRecorder
	mu           sync.Mutex
	grpcRequests []grpcReqRecord
}

type grpcReqRecord struct {
	method      string
	network     string
	containerID string
	podName     string
	err         error
}

func (r *testMetricsRecorder) RecordGRPCRequest(method, network, containerID, podName string, err error, duration time.Duration) {
	r.mu.Lock()
	r.grpcRequests = append(r.grpcRequests, grpcReqRecord{
		method:      method,
		network:     network,
		containerID: containerID,
		podName:     podName,
		err:         err,
	})
	r.mu.Unlock()
	if r.MetricsRecorder != nil {
		r.MetricsRecorder.RecordGRPCRequest(method, network, containerID, podName, err, duration)
	}
}

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

	rec := &testMetricsRecorder{MetricsRecorder: metrics.NewPrometheusRecorder()}
	server := newAdaptiveIpamServer(logger, s, sockPath, 0, 0, rec)
	defer server.stop()

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

	conn, err := grpc.NewClient("unix://"+sockPath, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(_ context.Context, addr string) (net.Conn, error) {
		addr = strings.TrimPrefix(addr, "unix://")
		return net.Dial("unix", addr)
	}))
	if err != nil {
		t.Fatalf("Failed to dial UDS %s: %v", sockPath, err)
	}
	defer conn.Close()

	client := adaptiveipam.NewAdaptiveIpamClient(conn)

	// 3. Prepare data and call AllocatePodIP
	network := "integration-network"
	cidr := "10.0.1.0/24"
	containerID := "test-container-integration"
	podName := "test-pod"
	req := &adaptiveipam.AllocatePodIPRequest{
		Network:      network,
		PodName:      podName,
		PodNamespace: "default",
		Ipv4Config: &adaptiveipam.IPConfig{
			InterfaceName:  "eth0",
			ContainerId:    containerID,
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

	// 4. Test CheckPodIP via gRPC client
	checkReq := &adaptiveipam.CheckPodIPRequest{
		Network:       network,
		InterfaceName: "eth0",
		ContainerId:   containerID,
		PodName:       podName,
		PodNamespace:  "default",
	}
	if _, err := client.CheckPodIP(ctx, checkReq); err != nil {
		t.Errorf("gRPC Client CheckPodIP failed: %v", err)
	}

	// 5. Test DeallocatePodIP via gRPC client
	deallocReq := &adaptiveipam.DeallocatePodIPRequest{
		Network:       network,
		InterfaceName: "eth0",
		ContainerId:   containerID,
		PodName:       podName,
		PodNamespace:  "default",
	}
	if _, err := client.DeallocatePodIP(ctx, deallocReq); err != nil {
		t.Errorf("gRPC Client DeallocatePodIP failed: %v", err)
	}

	// 6. Verify interceptor recorded metrics on rec & Prometheus counters via testutil.ToFloat64
	rec.mu.Lock()
	reqs := rec.grpcRequests
	rec.mu.Unlock()

	if len(reqs) != 3 {
		t.Fatalf("Expected 3 recorded gRPC requests from interceptor, got %d", len(reqs))
	}
	expectedMethods := []string{"AllocatePodIP", "CheckPodIP", "DeallocatePodIP"}
	for i, expMethod := range expectedMethods {
		got := reqs[i]
		if got.method != expMethod || got.network != network || got.containerID != containerID || got.podName != podName || got.err != nil {
			t.Errorf("Request %d: unexpected recorded gRPC request: %+v (expected method %s)", i, got, expMethod)
		}

		count := testutil.ToFloat64(metrics.GRPCServerHandledTotal.WithLabelValues(expMethod, "OK", network, containerID, podName))
		if count < 1.0 {
			t.Errorf("Expected Prometheus counter metis_daemon_grpc_server_handled_total to be >= 1 for method %s, got %v", expMethod, count)
		}
	}
}
