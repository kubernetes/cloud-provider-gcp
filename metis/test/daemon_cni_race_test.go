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

package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	nncv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/nodenetworkconfig/v1"
	nncfake "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/clientset/versioned/fake"
	"github.com/containernetworking/cni/pkg/skel"
	current "github.com/containernetworking/cni/pkg/types/100"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/metis/pkg/cni"
	"k8s.io/metis/pkg/daemon"
)

// Helper to run CNI CmdAdd with captured stdout
func runCNICmdAdd(plugin *cni.Plugin, args *skel.CmdArgs) (*current.Result, error) {
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	outChan := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		outChan <- buf.String()
	}()

	err := plugin.CmdAdd(args)

	w.Close()
	os.Stdout = oldStdout
	stdoutStr := <-outChan
	r.Close()

	if err != nil {
		return nil, err
	}

	var result current.Result
	if unmarshalErr := json.Unmarshal([]byte(stdoutStr), &result); unmarshalErr != nil {
		return nil, fmt.Errorf("CNI result unmarshal error: %w (stdout: %q)", unmarshalErr, stdoutStr)
	}

	return &result, nil
}

func setupDaemon(t *testing.T, dbPath, socketPath string) (*daemon.Daemon, context.Context, context.CancelFunc) {
	t.Setenv("NODE_NAME", "test-node")

	daemonConfig := daemon.Config{
		DBPath:          dbPath,
		SocketPath:      socketPath,
		MonitorInterval: 2 * time.Second,
		ReleaseCooldown: 1 * time.Minute,
	}

	d := daemon.NewDaemon(daemonConfig)
	d.NNCClient = nncfake.NewSimpleClientset(&nncv1.NodeNetworkConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node",
		},
	})
	d.KubeClient = kubefake.NewSimpleClientset(&corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-node",
		},
	})

	daemonCtx, daemonCancel := context.WithCancel(context.Background())
	return d, daemonCtx, daemonCancel
}

// Sequence A: CNI runs BEFORE Daemon starts.
// CNI falls back to direct mode, initializes SQLite, and allocates an IP.
// Then Daemon starts up, opens the existing SQLite DB, and processes subsequent gRPC requests cleanly.
func TestCNI_SequenceA_CNIBeforeDaemon(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "seq_a.sqlite")
	socketPath := filepath.Join(tempDir, "seq_a.sock")
	logFile := filepath.Join(tempDir, "seq_a.log")

	cniPlugin := cni.NewPlugin(
		cni.WithSocketPath(socketPath),
		cni.WithDBPath(dbPath),
		cni.WithLogFile(logFile),
	)

	// 1. Run CNI before daemon exists
	args1 := &skel.CmdArgs{
		ContainerID: "container-a1",
		Netns:       "/var/run/netns/test",
		IfName:      "eth0",
		Args:        "K8S_POD_NAME=pod-a1;K8S_POD_NAMESPACE=test-ns",
		StdinData:   []byte(`{"cniVersion": "0.4.0", "name": "test-net", "type": "metis", "ipam": {"type": "metis", "ranges": [[{"subnet": "10.240.0.0/24"}]], "routes": [{"dst": "0.0.0.0/0"}]}}`),
	}

	res1, err := runCNICmdAdd(cniPlugin, args1)
	if err != nil {
		t.Fatalf("Sequence A: Initial CNI direct fallback failed: %v", err)
	}
	if len(res1.IPs) == 0 {
		t.Fatalf("Sequence A: Expected IP allocation, got 0")
	}

	// 2. Now start the Daemon on the same DB & socket
	d, daemonCtx, cancel := setupDaemon(t, dbPath, socketPath)
	defer cancel()

	go func() {
		_ = d.Run(daemonCtx)
	}()

	// Wait for daemon socket to be created
	var socketReady bool
	for i := 0; i < 20; i++ {
		if _, statErr := os.Stat(socketPath); statErr == nil {
			socketReady = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !socketReady {
		t.Fatalf("Sequence A: Daemon failed to create socket within 1 second")
	}

	// 3. Run CNI again now that daemon is running
	args2 := &skel.CmdArgs{
		ContainerID: "container-a2",
		Netns:       "/var/run/netns/test",
		IfName:      "eth0",
		Args:        "K8S_POD_NAME=pod-a2;K8S_POD_NAMESPACE=test-ns",
		StdinData:   []byte(`{"cniVersion": "0.4.0", "name": "test-net", "type": "metis", "ipam": {"type": "metis", "ranges": [[{"subnet": "10.240.0.0/24"}]], "routes": [{"dst": "0.0.0.0/0"}]}}`),
	}

	res2, err := runCNICmdAdd(cniPlugin, args2)
	if err != nil {
		t.Fatalf("Sequence A: CNI via running daemon failed: %v", err)
	}
	if len(res2.IPs) == 0 {
		t.Fatalf("Sequence A: Expected IP allocation from daemon, got 0")
	}
}

// Sequence B: Socket becomes available DURING CNI connection retry loop (Socket Retry -> Daemon Mode).
func TestCNI_SequenceB_SocketAppearsDuringRetry(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "seq_b.sqlite")
	socketPath := filepath.Join(tempDir, "seq_b.sock")
	logFile := filepath.Join(tempDir, "seq_b.log")

	cniPlugin := cni.NewPlugin(
		cni.WithSocketPath(socketPath),
		cni.WithDBPath(dbPath),
		cni.WithLogFile(logFile),
	)

	// Start daemon with a 150ms delay before socket creation
	d, daemonCtx, cancel := setupDaemon(t, dbPath, socketPath)
	defer cancel()

	go func() {
		time.Sleep(150 * time.Millisecond)
		_ = d.Run(daemonCtx)
	}()

	args := &skel.CmdArgs{
		ContainerID: "container-b1",
		Netns:       "/var/run/netns/test",
		IfName:      "eth0",
		Args:        "K8S_POD_NAME=pod-b1;K8S_POD_NAMESPACE=test-ns",
		StdinData:   []byte(`{"cniVersion": "0.4.0", "name": "test-net", "type": "metis", "ipam": {"type": "metis", "ranges": [[{"subnet": "10.240.0.0/24"}]], "routes": [{"dst": "0.0.0.0/0"}]}}`),
	}

	// CNI retries 3 times with 100ms delay. Since daemon socket appears after 150ms,
	// attempt 0 (0ms) fails, attempt 1 (100ms) fails, attempt 2 (200ms) succeeds over daemon gRPC!
	res, err := runCNICmdAdd(cniPlugin, args)
	if err != nil {
		t.Fatalf("Sequence B: CNI socket retry connection failed: %v", err)
	}
	if len(res.IPs) == 0 {
		t.Fatalf("Sequence B: Expected IP allocation, got 0")
	}
}

// Sequence C: Daemon is fully running before CNI starts.
func TestCNI_SequenceC_DaemonAlreadyRunning(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "seq_c.sqlite")
	socketPath := filepath.Join(tempDir, "seq_c.sock")
	logFile := filepath.Join(tempDir, "seq_c.log")

	d, daemonCtx, cancel := setupDaemon(t, dbPath, socketPath)
	defer cancel()

	go func() {
		_ = d.Run(daemonCtx)
	}()

	// Wait for socket to exist
	for i := 0; i < 20; i++ {
		if _, statErr := os.Stat(socketPath); statErr == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	cniPlugin := cni.NewPlugin(
		cni.WithSocketPath(socketPath),
		cni.WithDBPath(dbPath),
		cni.WithLogFile(logFile),
	)

	args := &skel.CmdArgs{
		ContainerID: "container-c1",
		Netns:       "/var/run/netns/test",
		IfName:      "eth0",
		Args:        "K8S_POD_NAME=pod-c1;K8S_POD_NAMESPACE=test-ns",
		StdinData:   []byte(`{"cniVersion": "0.4.0", "name": "test-net", "type": "metis", "ipam": {"type": "metis", "ranges": [[{"subnet": "10.240.0.0/24"}]], "routes": [{"dst": "0.0.0.0/0"}]}}`),
	}

	res, err := runCNICmdAdd(cniPlugin, args)
	if err != nil {
		t.Fatalf("Sequence C: CNI via running daemon failed: %v", err)
	}
	if len(res.IPs) == 0 {
		t.Fatalf("Sequence C: Expected IP allocation, got 0")
	}
}
