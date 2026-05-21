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

package kops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateSSHKey(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ssh-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	privPath := filepath.Join(tmpDir, "id_rsa")
	pubPath := privPath + ".pub"

	if err := generateSSHKey(privPath, pubPath); err != nil {
		t.Fatalf("generateSSHKey failed: %v", err)
	}

	if _, err := os.Stat(privPath); err != nil {
		t.Errorf("private key not found: %v", err)
	}
	if _, err := os.Stat(pubPath); err != nil {
		t.Errorf("public key not found: %v", err)
	}

	// Read keys and verify format
	privBytes, err := os.ReadFile(privPath)
	if err != nil {
		t.Fatalf("failed to read private key: %v", err)
	}
	if !strings.Contains(string(privBytes), "BEGIN RSA PRIVATE KEY") {
		t.Errorf("invalid private key format: %s", string(privBytes))
	}

	pubBytes, err := os.ReadFile(pubPath)
	if err != nil {
		t.Fatalf("failed to read public key: %v", err)
	}
	if !strings.Contains(string(pubBytes), "ssh-rsa") {
		t.Errorf("invalid public key format: %s", string(pubBytes))
	}
	if !strings.Contains(string(pubBytes), "@kops") {
		t.Errorf("public key missing comment: %s", string(pubBytes))
	}
}
