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

func TestHydrateTemplate(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "kops-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	templateSrc := filepath.Join(tmpDir, "template.yaml")
	templateContent := `
project: ${GCP_PROJECT}
location: ${GCP_LOCATION}
zones: ${GCP_ZONES}
nodes: ${NODE_COUNT}
kops: {{ .clusterName }}
`
	if err := os.WriteFile(templateSrc, []byte(templateContent), 0644); err != nil {
		t.Fatalf("failed to write template source: %v", err)
	}

	config := &Config{
		GCPProject:   "test-project",
		GCPLocation:  "test-location",
		GCPZones:     "test-zone-1,test-zone-2",
		NodeCount:    3,
		TemplateSrc:  templateSrc,
		TemplatePath: filepath.Join(tmpDir, "hydrated.yaml"),
	}

	if err := HydrateTemplate(config); err != nil {
		t.Fatalf("HydrateTemplate failed: %v", err)
	}

	hydratedContent, err := os.ReadFile(config.TemplatePath)
	if err != nil {
		t.Fatalf("failed to read hydrated template: %v", err)
	}

	expected := `
project: test-project
location: test-location
zones: test-zone-1,test-zone-2
nodes: 3
kops: {{ .clusterName }}
`
	if string(hydratedContent) != expected {
		t.Errorf("Hydrated content mismatch.\nExpected: %s\nGot: %s", expected, string(hydratedContent))
	}
}

func TestConfigFinalize(t *testing.T) {
	// We assume we are running from the repo
	repo, err := repoRoot()
	if err != nil {
		t.Fatalf("failed to find repo root: %v", err)
	}
	workspace := repo

	c := &Config{
		ClusterName: "custom.k8s.local",
	}

	if err := c.Finalize(); err != nil {
		t.Fatalf("Finalize failed: %v", err)
	}

	// Verify defaults
	if c.GCPLocation != "us-central1" {
		t.Errorf("expected GCPLocation us-central1, got %s", c.GCPLocation)
	}
	if c.NodeCount != 2 {
		t.Errorf("expected NodeCount 2, got %d", c.NodeCount)
	}

	// Verify derived paths
	expectedWorkDir := filepath.Join(workspace, "clusters", "custom.k8s.local")
	if !strings.Contains(c.TemplatePath, expectedWorkDir) {
		t.Errorf("expected TemplatePath to contain %s, got %s", expectedWorkDir, c.TemplatePath)
	}
	if !strings.Contains(c.SSHPrivateKey, expectedWorkDir) {
		t.Errorf("expected SSHPrivateKey to contain %s, got %s", expectedWorkDir, c.SSHPrivateKey)
	}
	if c.SSHPublicKey != c.SSHPrivateKey+".pub" {
		t.Errorf("expected SSHPublicKey to be SSHPrivateKey + .pub, got %s", c.SSHPublicKey)
	}
}
