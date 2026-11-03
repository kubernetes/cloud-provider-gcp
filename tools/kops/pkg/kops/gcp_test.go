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

	"cloud.google.com/go/compute/apiv1/computepb"
)

func TestGenerateSSHKey(t *testing.T) {
	tmpDir := t.TempDir()

	privPath := filepath.Join(tmpDir, "id_rsa")
	pubPath := privPath + ".pub"

	if err := generateSSHKey(privPath, pubPath); err != nil {
		t.Fatalf("generateSSHKey failed: %v", err)
	}

	privInfo, err := os.Stat(privPath)
	if err != nil {
		t.Fatalf("private key not found: %v", err)
	}
	if got := privInfo.Mode().Perm(); got != 0600 {
		t.Errorf("expected private key permissions 0600, got %04o", got)
	}

	pubInfo, err := os.Stat(pubPath)
	if err != nil {
		t.Fatalf("public key not found: %v", err)
	}
	if got := pubInfo.Mode().Perm(); got != 0644 {
		t.Errorf("expected public key permissions 0644, got %04o", got)
	}

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
	tmpDir := t.TempDir()
	credsPath := filepath.Join(tmpDir, "dummy-creds.json")

	dummySA := "test-discover-sa@dummy-project.iam.gserviceaccount.com"
	jsonContent := fmt.Sprintf(`{"type": "service_account", "client_email": "%s"}`, dummySA)
	if err := os.WriteFile(credsPath, []byte(jsonContent), 0600); err != nil {
		t.Fatalf("failed to write temp credentials file: %v", err)
	}

	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", credsPath)

	ctx := context.Background()
	sa, err := discoverAccount(ctx)
	if err != nil {
		t.Fatalf("discoverAccount failed: %v", err)
	}

	if sa != dummySA {
		t.Errorf("expected discovered account to be %s, got %s", dummySA, sa)
	}
}

func TestIsValidEmail(t *testing.T) {
	tests := []struct {
		email string
		want  bool
	}{
		{"test-sa@project.iam.gserviceaccount.com", true},
		{"user@example.com", true},
		{"k8s-infra-prow-build.svc.id.goog", false},
		{"", false},
		{"@domain.com", false},
		{"user@", false},
		{"user @domain.com", false},
	}

	for _, tc := range tests {
		if got := isValidEmail(tc.email); got != tc.want {
			t.Errorf("isValidEmail(%q) = %v, want %v", tc.email, got, tc.want)
		}
	}
}

func TestSSHUsername(t *testing.T) {
	t.Run("prefers KUBE_SSH_USER over USER", func(t *testing.T) {
		t.Setenv("KUBE_SSH_USER", "kube-admin")
		t.Setenv("USER", "local-user")
		if got := sshUsername(); got != "kube-admin" {
			t.Errorf("sshUsername() = %q, want %q", got, "kube-admin")
		}
	})

	t.Run("falls back to USER", func(t *testing.T) {
		t.Setenv("KUBE_SSH_USER", "")
		t.Setenv("USER", "local-user")
		if got := sshUsername(); got != "local-user" {
			t.Errorf("sshUsername() = %q, want %q", got, "local-user")
		}
	})

	t.Run("defaults to kops", func(t *testing.T) {
		t.Setenv("KUBE_SSH_USER", "")
		t.Setenv("USER", "")
		if got := sshUsername(); got != "kops" {
			t.Errorf("sshUsername() = %q, want %q", got, "kops")
		}
	})
}

func TestAddAndRemoveSSHKeyMetadata(t *testing.T) {
	fp := "fingerprint-123"
	md := &computepb.Metadata{
		Fingerprint: &fp,
	}

	key1 := "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQ1 user1@kops\n"
	key2 := "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQ2 user2@kops\n"

	md, updated := addSSHKeyToMetadata(md, "user1", key1)
	if !updated {
		t.Fatalf("expected metadata to be updated when adding first key")
	}
	if md.Fingerprint == nil || *md.Fingerprint != fp {
		t.Errorf("expected fingerprint %q to be preserved, got %v", fp, md.Fingerprint)
	}

	// Adding duplicate key should be a no-op
	md, updated = addSSHKeyToMetadata(md, "user1", key1)
	if updated {
		t.Errorf("expected duplicate key addition to be a no-op")
	}

	// Adding second key should append
	md, updated = addSSHKeyToMetadata(md, "user2", key2)
	if !updated {
		t.Fatalf("expected metadata to be updated when adding second key")
	}

	// Removing first key should leave second key intact
	if !removeSSHKeyFromMetadata(md, key1) {
		t.Fatalf("expected removeSSHKeyFromMetadata to return true for existing key")
	}
	if len(md.Items) != 1 || md.Items[0].Value == nil {
		t.Fatalf("unexpected metadata items after removal: %+v", md.Items)
	}
	if got, want := *md.Items[0].Value, "user2:ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQ2 user2@kops"; got != want {
		t.Errorf("got ssh-keys %q, want %q", got, want)
	}

	// Removing non-existent or empty key should return false
	if removeSSHKeyFromMetadata(md, key1) {
		t.Errorf("expected false when removing already-removed key")
	}
	if removeSSHKeyFromMetadata(md, "   ") {
		t.Errorf("expected false when removing empty key")
	}
}
