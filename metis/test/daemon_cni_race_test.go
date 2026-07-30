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
	"sync"
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

// TestDaemonAndCNIStartupRace verifies concurrent startup behavior when the Daemon and CNI
// attempt to initialize the SQLite database and allocate IPs at the same time.
func TestDaemonAndCNIStartupRace(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "race_test.sqlite")
	socketPath := filepath.Join(tempDir, "race_test.sock")
	logFile := filepath.Join(tempDir, "metis-cni.log")

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
	defer daemonCancel()

	var wg sync.WaitGroup

	// 1. Start the Daemon in the background
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := d.Run(daemonCtx); err != nil {
			t.Logf("Daemon exited: %v", err)
		}
	}()

	// 2. Concurrently attempt CNI allocations (simulating CNI running while Daemon is setting up DB)
	cniPlugin := cni.NewPlugin(
		cni.WithSocketPath(socketPath),
		cni.WithDBPath(dbPath),
		cni.WithLogFile(logFile),
	)

	var stdoutMu sync.Mutex
	cniErrCh := make(chan error, 5)

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			containerID := fmt.Sprintf("container-%d", id)
			args := &skel.CmdArgs{
				ContainerID: containerID,
				Netns:       "/var/run/netns/test",
				IfName:      "eth0",
				Args:        fmt.Sprintf("K8S_POD_NAME=pod-%d;K8S_POD_NAMESPACE=test-ns", id),
				StdinData:   []byte(`{"cniVersion": "0.4.0", "name": "test-net", "type": "metis", "ipam": {"type": "metis", "ranges": [[{"subnet": "10.240.0.0/24"}]], "routes": [{"dst": "0.0.0.0/0"}]}}`),
			}

			stdoutMu.Lock()
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			outChan := make(chan string)
			go func() {
				var buf bytes.Buffer
				_, _ = io.Copy(&buf, r)
				outChan <- buf.String()
			}()

			err := cniPlugin.CmdAdd(args)

			w.Close()
			os.Stdout = oldStdout
			stdoutStr := <-outChan
			r.Close()
			stdoutMu.Unlock()

			if err != nil {
				cniErrCh <- fmt.Errorf("CNI CmdAdd failed for pod-%d: %w", id, err)
				return
			}

			var result current.Result
			if unmarshalErr := json.Unmarshal([]byte(stdoutStr), &result); unmarshalErr != nil {
				cniErrCh <- fmt.Errorf("CNI result unmarshal failed for pod-%d: %w (output: %q)", id, unmarshalErr, stdoutStr)
				return
			}

			if len(result.IPs) == 0 {
				cniErrCh <- fmt.Errorf("CNI result had 0 IPs for pod-%d", id)
				return
			}
		}(i)
	}

	// Stop daemon after tests finish
	go func() {
		time.Sleep(1 * time.Second)
		daemonCancel()
	}()

	wg.Wait()
	close(cniErrCh)

	for err := range cniErrCh {
		t.Errorf("Race test failure: %v", err)
	}
}
