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
	"context"
	"fmt"
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

func TestDiscoverAccount(t *testing.T) {
	// Create a temporary JSON file representing a service account key
	tmpFile, err := os.CreateTemp("", "dummy-creds-*.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	dummySA := "test-discover-sa@dummy-project.iam.gserviceaccount.com"
	jsonContent := fmt.Sprintf(`{"type": "service_account", "client_email": "%s"}`, dummySA)
	if _, err := tmpFile.Write([]byte(jsonContent)); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	tmpFile.Close()

	// Backup existing env var
	origEnv := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	defer os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", origEnv)

	// Set env var to point to our dummy file
	os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", tmpFile.Name())

	ctx := context.Background()
	sa, err := discoverAccount(ctx)
	if err != nil {
		t.Fatalf("discoverAccount failed: %v", err)
	}

	if sa != dummySA {
		t.Errorf("expected discovered account to be %s, got %s", dummySA, sa)
	}
}

