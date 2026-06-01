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
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/containernetworking/cni/libcni"
	"k8s.io/apimachinery/pkg/util/wait"
)

func TestLibcniConformance(t *testing.T) {
	// 1. Create an isolated playground for the test lifecycle
	tempDir, err := os.MkdirTemp("", "libcni-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	binDir := filepath.Join(tempDir, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatal(err)
	}
	binPath := filepath.Join(binDir, "metis")
	socketPath := filepath.Join(tempDir, "metis.sock")
	dbPath := filepath.Join(tempDir, "metis.sqlite")

	// 2. Build the binary automatically inside the test to guarantee synchronization
	cmd := exec.Command("go", "build", "-o", binPath, "k8s.io/metis/cmd")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build metis CNI binary: %v\nOutput: %s", err, string(output))
	}
	// 3. Spin up the localized backend daemon as a subprocess
	daemonLogFile, err := os.Create(filepath.Join(tempDir, "daemon.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer daemonLogFile.Close()

	daemonCmd := exec.Command(binPath, "daemon", "--socket-path", socketPath, "--db-path", dbPath)
	daemonCmd.Stdout = daemonLogFile
	daemonCmd.Stderr = daemonLogFile

	if err := daemonCmd.Start(); err != nil {
		t.Fatalf("Failed to start daemon subprocess: %v", err)
	}
	defer func() {
		_ = daemonCmd.Process.Kill()
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Wait until the domain socket successfully mounts
	err = wait.PollUntilContextTimeout(ctx, 100*time.Millisecond, 5*time.Second, true, func(ctx context.Context) (bool, error) {
		if _, err := os.Stat(socketPath); err == nil {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		t.Fatalf("Metis Daemon did not become ready: %v", err)
	}

	// 4. Trigger rigorous validation natively via libcni
	cniLib := libcni.NewCNIConfig([]string{binDir}, nil)

	netConfigString := fmt.Sprintf(`{
		"cniVersion": "0.4.0",
		"name": "metis-network",
		"type": "metis",
		"daemonSocket": "%s",
		"logFile": "%s",
		"ipam": {
			"type": "metis",
			"ranges": [
				[{"subnet": "10.240.0.0/24"}]
			],
			"routes": [
				{"dst": "0.0.0.0/0"}
			]
		}
	}`, socketPath, filepath.Join(tempDir, "metis-cni.log"))

	conf, err := libcni.ConfFromBytes([]byte(netConfigString))
	if err != nil {
		t.Fatalf("Failed to decode internal libcni config bytes: %v", err)
	}

	runtimeConf := &libcni.RuntimeConf{
		ContainerID: "test-conformance-container",
		NetNS:       "/var/run/netns/test-ns",
		IfName:      "eth0",
		CacheDir:    tempDir,
		Args: [][2]string{
			{"K8S_POD_NAME", "test-pod"},
			{"K8S_POD_NAMESPACE", "default"},
		},
	}

	// Validate ADD
	result, err := cniLib.AddNetwork(ctx, conf, runtimeConf)
	if err != nil {
		t.Fatalf("libcni.AddNetwork failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil CNI Result upon AddNetwork completion")
	}

	resultBytes, _ := json.Marshal(result)
	checkConfigString := fmt.Sprintf(`{
		"cniVersion": "0.4.0",
		"name": "metis-network",
		"type": "metis",
		"daemonSocket": "%s",
		"logFile": "%s",
		"ipam": {
			"type": "metis",
			"ranges": [
				[{"subnet": "10.240.0.0/24"}]
			],
			"routes": [
				{"dst": "0.0.0.0/0"}
			]
		},
		"prevResult": %s
	}`, socketPath, filepath.Join(tempDir, "metis-cni.log"), string(resultBytes))
	checkConf, _ := libcni.ConfFromBytes([]byte(checkConfigString))

	// Validate CHECK (which we natively wired previously)
	if err := cniLib.CheckNetwork(ctx, checkConf, runtimeConf); err != nil {
		t.Fatalf("libcni.CheckNetwork failed: %v", err)
	}

	// Validate DEL
	if err := cniLib.DelNetwork(ctx, conf, runtimeConf); err != nil {
		t.Fatalf("libcni.DelNetwork failed: %v", err)
	}

	// Print captured logs before cleaning up
	if cniLogBytes, err := os.ReadFile(filepath.Join(tempDir, "metis-cni.log")); err == nil {
		t.Logf("=== CNI LOGS ===\n%s", string(cniLogBytes))
	}
	if daemonLogBytes, err := os.ReadFile(filepath.Join(tempDir, "daemon.log")); err == nil {
		t.Logf("=== DAEMON LOGS ===\n%s", string(daemonLogBytes))
	}
}

func TestInstallMode(t *testing.T) {
	// 1. Create isolated play directories
	tempDir, err := os.MkdirTemp("", "metis-install-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	testBinDir := filepath.Join(tempDir, "src-bin")
	if err := os.MkdirAll(testBinDir, 0755); err != nil {
		t.Fatal(err)
	}
	sourceBinPath := filepath.Join(testBinDir, "metis")

	// 2. Build the metis binary for testing
	cmd := exec.Command("go", "build", "-o", sourceBinPath, "k8s.io/metis/cmd")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build metis CNI binary for testing: %v\nOutput: %s", err, string(output))
	}

	// 3. Test 1: Installation via --cni-dir CLI Flag
	destCniDir1 := filepath.Join(tempDir, "cni-dir-1")
	installCmd1 := exec.Command(sourceBinPath, "install", "--cni-dir", destCniDir1)
	if output, err := installCmd1.CombinedOutput(); err != nil {
		t.Fatalf("Self-installer failed with --cni-dir flag: %v\nOutput: %s", err, string(output))
	}

	expectedBinPath1 := filepath.Join(destCniDir1, "bin", "metis")
	info1, err := os.Stat(expectedBinPath1)
	if err != nil {
		t.Fatalf("Failed to find installed binary at %q: %v", expectedBinPath1, err)
	}
	// Verify permissions: executable by user (0100)
	if info1.Mode().Perm()&0100 == 0 {
		t.Errorf("Installed binary at %q is not executable. Mode: %v", expectedBinPath1, info1.Mode())
	}
	// Verify that the temporary file used during atomic copy is cleanly deleted
	tempBinPath1 := expectedBinPath1 + ".tmp"
	if _, err := os.Stat(tempBinPath1); !os.IsNotExist(err) {
		t.Errorf("Expected staged temporary file at %q to be deleted, but it still exists", tempBinPath1)
	}

	// 4. Test 2: Installation via CNI_DIR Environment Variable (Fallback)
	destCniDir2 := filepath.Join(tempDir, "cni-dir-2")
	installCmd2 := exec.Command(sourceBinPath, "install")
	installCmd2.Env = append(os.Environ(), "CNI_DIR="+destCniDir2)
	if output, err := installCmd2.CombinedOutput(); err != nil {
		t.Fatalf("Self-installer failed using CNI_DIR env var: %v\nOutput: %s", err, string(output))
	}

	expectedBinPath2 := filepath.Join(destCniDir2, "bin", "metis")
	info2, err := os.Stat(expectedBinPath2)
	if err != nil {
		t.Fatalf("Failed to find installed binary via CNI_DIR env at %q: %v", expectedBinPath2, err)
	}
	if info2.Mode().Perm()&0100 == 0 {
		t.Errorf("Installed binary via CNI_DIR env at %q is not executable. Mode: %v", expectedBinPath2, info2.Mode())
	}
	// Verify that the temporary file used during atomic copy is cleanly deleted
	tempBinPath2 := expectedBinPath2 + ".tmp"
	if _, err := os.Stat(tempBinPath2); !os.IsNotExist(err) {
		t.Errorf("Expected staged temporary file via CNI_DIR env at %q to be deleted, but it still exists", tempBinPath2)
	}
}
