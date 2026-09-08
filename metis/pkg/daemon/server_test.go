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
	"testing"
	"time"

	"github.com/go-logr/logr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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

	server := newAdaptiveIpamServer(logger, s, sockPath, 0, 0)
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

	// 4. Test CheckPodIP via gRPC client
	checkReq := &adaptiveipam.CheckPodIPRequest{
		Network:       network,
		InterfaceName: "eth0",
		ContainerId:   "test-container-integration",
		PodName:       "test-pod",
		PodNamespace:  "default",
	}
	if _, err := client.CheckPodIP(ctx, checkReq); err != nil {
		t.Errorf("gRPC Client CheckPodIP failed: %v", err)
	}

	// 5. Test DeallocatePodIP via gRPC client
	deallocReq := &adaptiveipam.DeallocatePodIPRequest{
		Network:       network,
		InterfaceName: "eth0",
		ContainerId:   "test-container-integration",
		PodName:       "test-pod",
		PodNamespace:  "default",
	}
	if _, err := client.DeallocatePodIP(ctx, deallocReq); err != nil {
		t.Errorf("gRPC Client DeallocatePodIP failed: %v", err)
	}
}
