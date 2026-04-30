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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/containernetworking/cni/pkg/skel"
	current "github.com/containernetworking/cni/pkg/types/100"
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
	cases := []struct {
		name           string
		stdinData      string
		mockResp       *pb.AllocatePodIPResponse
		mockErr        error
		assertInput    func(t *testing.T, in *pb.AllocatePodIPRequest)
		expectErr      bool
		expectedIPs    []string
		expectedGWs    []string
		expectedRoutes []string
	}{
		{
			name:      "IPv4 only",
			stdinData: `{"cniVersion": "0.4.0", "name": "test-net", "type": "metis", "ipam": {"type": "metis", "ranges": [[{"subnet": "10.240.0.0/24"}]], "routes": [{"dst": "0.0.0.0/0"}]}}`,
			mockResp: &pb.AllocatePodIPResponse{
				Ipv4: &pb.PodIP{
					IpAddress: "10.240.0.2",
					Cidr:      "10.240.0.0/24",
				},
			},
			expectedIPs:    []string{"10.240.0.2/24"},
			expectedGWs:    []string{"10.240.0.1"},
			expectedRoutes: []string{"0.0.0.0/0"},
		},
		{
			name:      "DualStack",
			stdinData: `{"cniVersion": "0.3.1", "name": "gke-pod-network", "type": "gke", "ipam": {"type": "host-local", "ranges": [[{"subnet":"10.160.7.0/24"}],[{"subnet":"2600:1900:4040:ae7:0:7::/112"}]], "routes": [{"dst": "0.0.0.0/0"},{"dst": "::/0"}]}}`,
			mockResp: &pb.AllocatePodIPResponse{
				Ipv4: &pb.PodIP{
					IpAddress: "10.160.7.2",
					Cidr:      "10.160.7.0/24",
				},
				Ipv6: &pb.PodIP{
					IpAddress: "2600:1900:4040:ae7:0:7::2",
					Cidr:      "2600:1900:4040:ae7:0:7::/112",
				},
			},
			assertInput: func(t *testing.T, in *pb.AllocatePodIPRequest) {
				if in.Ipv6Config == nil {
					t.Errorf("expected IPv6 config to be passed")
				}
			},
			expectedIPs:    []string{"10.160.7.2/24", "2600:1900:4040:ae7:0:7::2/112"},
			expectedGWs:    []string{"10.160.7.1", "2600:1900:4040:ae7:0:7::1"},
			expectedRoutes: []string{"0.0.0.0/0", "::/0"},
		},
		{
			name:      "IPv6 only",
			stdinData: `{"cniVersion": "0.3.1", "name": "gke-pod-network", "type": "gke", "ipam": {"type": "host-local", "ranges": [[{"subnet":"2600:1900:4040:ae7:0:7::/112"}]], "routes": [{"dst": "::/0"}]}}`,
			mockResp: &pb.AllocatePodIPResponse{
				Ipv6: &pb.PodIP{
					IpAddress: "2600:1900:4040:ae7:0:7::2",
					Cidr:      "2600:1900:4040:ae7:0:7::/112",
				},
			},
			assertInput: func(t *testing.T, in *pb.AllocatePodIPRequest) {
				if in.Ipv6Config == nil {
					t.Errorf("expected IPv6 config to be passed")
				}
			},
			expectedIPs:    []string{"2600:1900:4040:ae7:0:7::2/112"},
			expectedGWs:    []string{"2600:1900:4040:ae7:0:7::1"},
			expectedRoutes: []string{"::/0"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
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
				if tc.assertInput != nil {
					tc.assertInput(t, in)
				}
				return tc.mockResp, tc.mockErr
			}

			args := &skel.CmdArgs{
				ContainerID: "test-container-id",
				Netns:       "/var/run/netns/test",
				IfName:      "eth0",
				Args:        "K8S_POD_NAME=test-pod;K8S_POD_NAMESPACE=test-ns",
				StdinData:   []byte(tc.stdinData),
			}

			result, err := plugin.cmdAdd(args)

			if (err != nil) != tc.expectErr {
				t.Fatalf("cmdAdd failed: %v", err)
			}

			if !tc.expectErr {
				if result == nil {
					t.Fatal("Expected non-nil result, got nil")
				}

				// Assert IPs
				if len(result.IPs) != len(tc.expectedIPs) {
					t.Errorf("Expected %d IPs, got %d", len(tc.expectedIPs), len(result.IPs))
				}
				for i, expectedIP := range tc.expectedIPs {
					if i >= len(result.IPs) {
						break
					}
					expectedIPAddr, _, err := net.ParseCIDR(expectedIP)
					if err != nil {
						t.Fatalf("Failed to parse expected IP %s: %v", expectedIP, err)
					}
					if !result.IPs[i].Address.IP.Equal(expectedIPAddr) {
						t.Errorf("Expected IP %s, got %s", expectedIP, result.IPs[i].Address.IP)
					}
					if i < len(tc.expectedGWs) {
						expectedGW := net.ParseIP(tc.expectedGWs[i])
						if !result.IPs[i].Gateway.Equal(expectedGW) {
							t.Errorf("Expected GW %s, got %s", tc.expectedGWs[i], result.IPs[i].Gateway)
						}
					}
				}

				// Assert Routes
				if len(result.Routes) != len(tc.expectedRoutes) {
					t.Errorf("Expected %d routes, got %d", len(tc.expectedRoutes), len(result.Routes))
				}
				for i, expectedRoute := range tc.expectedRoutes {
					if i >= len(result.Routes) {
						break
					}
					if result.Routes[i].Dst.String() != expectedRoute {
						t.Errorf("Expected route dst %s, got %s", expectedRoute, result.Routes[i].Dst.String())
					}
				}
			}
		})
	}
}

func TestCmdDel(t *testing.T) {
	cases := []struct {
		name      string
		args      string
		stdinData string
	}{
		{
			name:      "Normal",
			args:      "K8S_POD_NAME=test-pod;K8S_POD_NAMESPACE=test-ns",
			stdinData: `{"cniVersion": "0.4.0", "name": "test-net", "type": "metis"}`,
		},
		{
			name:      "EmptyArgs",
			args:      "",
			stdinData: `{"cniVersion": "0.4.0", "name": "test-net", "type": "metis"}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
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
				Args:        tc.args,
				StdinData:   []byte(tc.stdinData),
			}

			err := plugin.cmdDel(args)

			if err != nil {
				t.Fatalf("cmdDel failed: %v", err)
			}

			if !deallocateCalled {
				t.Fatalf("DeallocatePodIP was not called")
			}
		})
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

	err := plugin.cmdCheck(args)

	if err != nil {
		t.Fatalf("cmdCheck failed: %v", err)
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
		StdinData:   []byte(`{"cniVersion": "0.4.0", "name": "test-net", "type": "metis", "ipam": {"type": "metis", "ranges": [[{"subnet": "10.240.0.0/24"}]], "routes": [{"dst": "0.0.0.0/0"}]}}`),
	}

	result, err := plugin.cmdAdd(args)
	if err != nil {
		t.Fatalf("cmdAdd failed with actual daemon: %v", err)
	}
	if result == nil {
		t.Fatal("Expected non-nil result from cmdAdd")
	}

	err = plugin.cmdCheck(args)
	if err != nil {
		t.Fatalf("cmdCheck failed with actual daemon: %v", err)
	}
}

func runWithOutputCapture(t *testing.T, f func() error) (stdout string, stderr string, err error) {
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	}()

	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	os.Stdout = wOut
	os.Stderr = wErr

	outC := make(chan string)
	errC := make(chan string)

	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, rOut)
		outC <- buf.String()
	}()

	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, rErr)
		errC <- buf.String()
	}()

	err = f()

	wOut.Close()
	wErr.Close()

	stdout = <-outC
	stderr = <-errC

	return stdout, stderr, err
}

func TestCmdAdd_CleanStdout(t *testing.T) {
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
		StdinData:   []byte(`{"cniVersion": "0.4.0", "name": "test-net", "type": "metis", "ipam": {"type": "metis", "ranges": [[{"subnet": "10.240.0.0/24"}]], "routes": [{"dst": "0.0.0.0/0"}]}}`),
	}

	stdout, stderr, err := runWithOutputCapture(t, func() error {
		return plugin.CmdAdd(args)
	})

	if err != nil {
		t.Fatalf("CmdAdd failed: %v", err)
	}

	if stderr != "" {
		t.Errorf("Expected empty stderr, got: %q", stderr)
	}

	var result current.Result
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Errorf("Stdout is not valid JSON, does not match schema, or has garbage: %v. Output was: %q", err, stdout)
	}
}
