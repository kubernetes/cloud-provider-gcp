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

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

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
	// Use a custom name to verify that the installer dynamically resolves it from os.Executable()
	sourceBinPath := filepath.Join(testBinDir, "metis-test-binary")

	// 2. Build the metis binary for testing
	cmd := exec.Command("go", "build", "-o", sourceBinPath, "k8s.io/metis/cmd")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build metis CNI binary for testing: %v\nOutput: %s", err, string(output))
	}

	// Ensure the built binary has recognizable custom permissions so we can verify permission copy.
	// Let's set it to 0700 (read, write, execute by owner only)
	if err := os.Chmod(sourceBinPath, 0700); err != nil {
		t.Fatalf("Failed to chmod source test binary to 0700: %v", err)
	}
	sourceInfo, err := os.Stat(sourceBinPath)
	if err != nil {
		t.Fatal(err)
	}

	// 3. Test 1: Installation via --cni-dir CLI Flag
	destCniDir1 := filepath.Join(tempDir, "cni-dir-1")
	installCmd1 := exec.Command(sourceBinPath, "install", "--cni-dir", destCniDir1)
	if output, err := installCmd1.CombinedOutput(); err != nil {
		t.Fatalf("Self-installer failed with --cni-dir flag: %v\nOutput: %s", err, string(output))
	}

	expectedBinPath1 := filepath.Join(destCniDir1, "bin", "metis-test-binary")
	info1, err := os.Stat(expectedBinPath1)
	if err != nil {
		t.Fatalf("Failed to find installed binary at %q: %v", expectedBinPath1, err)
	}
	// Verify permissions: must match source permissions exactly (0700)
	if info1.Mode().Perm() != sourceInfo.Mode().Perm() {
		t.Errorf("Installed binary permissions mismatch. Expected %v, got %v", sourceInfo.Mode().Perm(), info1.Mode().Perm())
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

	expectedBinPath2 := filepath.Join(destCniDir2, "bin", "metis-test-binary")
	info2, err := os.Stat(expectedBinPath2)
	if err != nil {
		t.Fatalf("Failed to find installed binary via CNI_DIR env at %q: %v", expectedBinPath2, err)
	}
	if info2.Mode().Perm() != sourceInfo.Mode().Perm() {
		t.Errorf("Installed binary via CNI_DIR permissions mismatch. Expected %v, got %v", sourceInfo.Mode().Perm(), info2.Mode().Perm())
	}
	// Verify that the temporary file used during atomic copy is cleanly deleted
	tempBinPath2 := expectedBinPath2 + ".tmp"
	if _, err := os.Stat(tempBinPath2); !os.IsNotExist(err) {
		t.Errorf("Expected staged temporary file via CNI_DIR env at %q to be deleted, but it still exists", tempBinPath2)
	}
}
