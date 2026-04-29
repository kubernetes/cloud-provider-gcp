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

package cni

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/containernetworking/cni/pkg/skel"
	"google.golang.org/grpc"

	"k8s.io/apimachinery/pkg/util/wait"
	pb "k8s.io/metis/api/adaptiveipam/v1"
	"k8s.io/metis/pkg/daemon"
)

type mockAdaptiveIpamClient struct {
	pb.AdaptiveIpamClient
	allocatePodIPFunc   func(ctx context.Context, in *pb.AllocatePodIPRequest) (*pb.AllocatePodIPResponse, error)
	deallocatePodIPFunc func(ctx context.Context, in *pb.DeallocatePodIPRequest) (*pb.DeallocatePodIPResponse, error)
	checkPodIPFunc      func(ctx context.Context, in *pb.CheckPodIPRequest) (*pb.CheckPodIPResponse, error)
}

func (m *mockAdaptiveIpamClient) AllocatePodIP(ctx context.Context, in *pb.AllocatePodIPRequest, opts ...grpc.CallOption) (*pb.AllocatePodIPResponse, error) {
	if m.allocatePodIPFunc != nil {
		return m.allocatePodIPFunc(ctx, in)
	}
	return nil, fmt.Errorf("unimplemented")
}

func (m *mockAdaptiveIpamClient) DeallocatePodIP(ctx context.Context, in *pb.DeallocatePodIPRequest, opts ...grpc.CallOption) (*pb.DeallocatePodIPResponse, error) {
	if m.deallocatePodIPFunc != nil {
		return m.deallocatePodIPFunc(ctx, in)
	}
	return nil, fmt.Errorf("unimplemented")
}

func (m *mockAdaptiveIpamClient) CheckPodIP(ctx context.Context, in *pb.CheckPodIPRequest, opts ...grpc.CallOption) (*pb.CheckPodIPResponse, error) {
	if m.checkPodIPFunc != nil {
		return m.checkPodIPFunc(ctx, in)
	}
	return nil, fmt.Errorf("unimplemented")
}

func TestCmdAdd(t *testing.T) {
	mockClient := &mockAdaptiveIpamClient{}
	tempLogDir := t.TempDir()
	logFile := filepath.Join(tempLogDir, "metis-cni.log")

	plugin := NewPlugin(
		WithClientFunc(func(socketPath string) (pb.AdaptiveIpamClient, *grpc.ClientConn, error) {
			return mockClient, nil, nil
		}),
		WithLogFile(logFile),
	)

	mockClient.allocatePodIPFunc = func(ctx context.Context, in *pb.AllocatePodIPRequest) (*pb.AllocatePodIPResponse, error) {
		return &pb.AllocatePodIPResponse{
			Ipv4: &pb.PodIP{
				IpAddress: "10.240.0.2",
				Cidr:      "10.240.0.0/24",
			},
		}, nil
	}

	args := &skel.CmdArgs{
		ContainerID: "test-container-id",
		Netns:       "/var/run/netns/test",
		IfName:      "eth0",
		Args:        "K8S_POD_NAME=test-pod;K8S_POD_NAMESPACE=test-ns",
		StdinData:   []byte(`{"cniVersion": "0.4.0", "name": "test-net", "type": "metis", "ipam": {"type": "metis", "ranges": [[{"subnet": "10.240.0.0/24"}]]}}`),
	}

	err := plugin.CmdAdd(args)
	if err != nil {
		t.Fatalf("CmdAdd failed: %v", err)
	}
}

func TestCmdDel(t *testing.T) {
	mockClient := &mockAdaptiveIpamClient{}
	tempLogDir := t.TempDir()
	logFile := filepath.Join(tempLogDir, "metis-cni.log")

	plugin := NewPlugin(
		WithClientFunc(func(socketPath string) (pb.AdaptiveIpamClient, *grpc.ClientConn, error) {
			return mockClient, nil, nil
		}),
		WithLogFile(logFile),
	)

	deallocateCalled := false
	mockClient.deallocatePodIPFunc = func(ctx context.Context, in *pb.DeallocatePodIPRequest) (*pb.DeallocatePodIPResponse, error) {
		deallocateCalled = true
		return &pb.DeallocatePodIPResponse{}, nil
	}

	args := &skel.CmdArgs{
		ContainerID: "test-container-id",
		Netns:       "/var/run/netns/test",
		IfName:      "eth0",
		Args:        "K8S_POD_NAME=test-pod;K8S_POD_NAMESPACE=test-ns",
		StdinData:   []byte(`{"cniVersion": "0.4.0", "name": "test-net", "type": "metis"}`),
	}

	err := plugin.CmdDel(args)
	if err != nil {
		t.Fatalf("CmdDel failed: %v", err)
	}

	if !deallocateCalled {
		t.Fatalf("DeallocatePodIP was not called")
	}
}

func TestCmdCheck(t *testing.T) {
	mockClient := &mockAdaptiveIpamClient{}
	tempLogDir := t.TempDir()
	logFile := filepath.Join(tempLogDir, "metis-cni.log")

	plugin := NewPlugin(
		WithClientFunc(func(socketPath string) (pb.AdaptiveIpamClient, *grpc.ClientConn, error) {
			return mockClient, nil, nil
		}),
		WithLogFile(logFile),
	)

	checkCalled := false
	mockClient.checkPodIPFunc = func(ctx context.Context, in *pb.CheckPodIPRequest) (*pb.CheckPodIPResponse, error) {
		checkCalled = true
		return &pb.CheckPodIPResponse{}, nil
	}

	args := &skel.CmdArgs{
		ContainerID: "test-container-id",
		Netns:       "/var/run/netns/test",
		IfName:      "eth0",
		Args:        "K8S_POD_NAME=test-pod;K8S_POD_NAMESPACE=test-ns",
		StdinData:   []byte(`{"cniVersion": "0.4.0", "name": "test-net", "type": "metis"}`),
	}

	err := plugin.CmdCheck(args)
	if err != nil {
		t.Fatalf("CmdCheck failed: %v", err)
	}

	if !checkCalled {
		t.Fatalf("CheckPodIP was not called")
	}
}

func TestCniWithActualDaemon(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "metis-e2e-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	socketPath := filepath.Join(tempDir, "metis.sock")
	dbPath := filepath.Join(tempDir, "metis.sqlite")

	logFile := filepath.Join(tempDir, "metis-cni.log")

	plugin := NewPlugin(
		WithClientFunc(func(path string) (pb.AdaptiveIpamClient, *grpc.ClientConn, error) {
			return getGrpcClient(socketPath)
		}),
		WithSocketPath(socketPath),
		WithLogFile(logFile),
	)

	cfg := daemon.Config{
		DBPath:     dbPath,
		SocketPath: socketPath,
	}
	d := daemon.NewDaemon(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()

	// Wait for socket to be created
	err = wait.PollUntilContextTimeout(context.Background(), 100*time.Millisecond, 5*time.Second, true, func(ctx context.Context) (bool, error) {
		if _, err := os.Stat(socketPath); err == nil {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		t.Fatalf("Daemon failed to start and create socket: %v", err)
	}

	args := &skel.CmdArgs{
		ContainerID: "test-container-id",
		Netns:       "/var/run/netns/test",
		IfName:      "eth0",
		Args:        "K8S_POD_NAME=test-pod;K8S_POD_NAMESPACE=test-ns",
		StdinData:   []byte(`{"cniVersion": "0.4.0", "name": "test-net", "type": "metis", "ipam": {"type": "metis", "ranges": [[{"subnet": "10.240.0.0/24"}]]}}`),
	}

	err = plugin.CmdAdd(args)
	if err != nil {
		t.Fatalf("CmdAdd failed with actual daemon: %v", err)
	}

	err = plugin.CmdCheck(args)
	if err != nil {
		t.Fatalf("CmdCheck failed with actual daemon: %v", err)
	}
}
