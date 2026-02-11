package kops

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestHydrateTemplate(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "kops-test")
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
	if err := ioutil.WriteFile(templateSrc, []byte(templateContent), 0644); err != nil {
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

	hydratedContent, err := ioutil.ReadFile(config.TemplatePath)
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

func TestNewConfigFromEnv(t *testing.T) {
	// Save current env
	oldClusterName := os.Getenv("CLUSTER_NAME")
	oldProject := os.Getenv("GCP_PROJECT")
	oldNodeCount := os.Getenv("NODE_COUNT")
	defer func() {
		os.Setenv("CLUSTER_NAME", oldClusterName)
		os.Setenv("GCP_PROJECT", oldProject)
		os.Setenv("NODE_COUNT", oldNodeCount)
	}()

	os.Setenv("CLUSTER_NAME", "my-test-cluster")
	os.Setenv("GCP_PROJECT", "my-test-project")
	os.Setenv("NODE_COUNT", "5")

	config, err := NewConfigFromEnv()
	if err != nil {
		t.Fatalf("NewConfigFromEnv failed: %v", err)
	}

	if config.ClusterName != "my-test-cluster" {
		t.Errorf("expected cluster name my-test-cluster, got %s", config.ClusterName)
	}
	if config.GCPProject != "my-test-project" {
		t.Errorf("expected project my-test-project, got %s", config.GCPProject)
	}
	if config.NodeCount != 5 {
		t.Errorf("expected node count 5, got %d", config.NodeCount)
	}
}
