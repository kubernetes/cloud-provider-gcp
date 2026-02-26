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
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config holds all parameters for kOps cluster lifecycle.
type Config struct {
	ClusterName             string
	GCPProject              string
	GCPLocation             string
	GCPZones                string
	K8sVersion              string
	KopsBin                 string
	KopsBaseURL             string
	StateStore              string
	TemplateSrc             string
	TemplatePath            string
	AdminAccess             string
	ControlPlaneMachineType string
	NodeMachineType         string
	NodeCount               int
	SSHPrivateKey           string
	SSHPublicKey            string // Derived internally
	KopsFeatureFlags        string
	GoogleAppCredentials    string
	NewCCMSpec              string
	ImageRepo               string
	ImageTag                string
	ValidationWait          time.Duration
}

// NewConfigFromEnv initializes Config with values from environment variables.
func NewConfigFromEnv() (*Config, error) {
	c := &Config{
		ClusterName:             os.Getenv("CLUSTER_NAME"),
		GCPProject:              os.Getenv("GCP_PROJECT"),
		GCPLocation:             os.Getenv("GCP_LOCATION"),
		GCPZones:                os.Getenv("GCP_ZONES"),
		K8sVersion:              os.Getenv("K8S_VERSION"),
		KopsBin:                 os.Getenv("KOPS_BIN"),
		KopsBaseURL:             os.Getenv("KOPS_BASE_URL"),
		StateStore:              os.Getenv("KOPS_STATE_STORE"),
		TemplateSrc:             os.Getenv("KOPS_TEMPLATE_SRC"),
		TemplatePath:            os.Getenv("KOPS_TEMPLATE"),
		AdminAccess:             os.Getenv("ADMIN_ACCESS"),
		ControlPlaneMachineType: os.Getenv("CONTROL_PLANE_MACHINE_TYPE"),
		NodeMachineType:         os.Getenv("NODE_MACHINE_TYPE"),
		SSHPrivateKey:           os.Getenv("SSH_PRIVATE_KEY"),
		KopsFeatureFlags:        os.Getenv("KOPS_FEATURE_FLAGS"),
		GoogleAppCredentials:    os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"),
		NewCCMSpec:              os.Getenv("NEW_CCM_SPEC"),
		ImageRepo:               os.Getenv("IMAGE_REPO"),
		ImageTag:                os.Getenv("IMAGE_TAG"),
	}

	if nodeCountStr := os.Getenv("NODE_COUNT"); nodeCountStr != "" {
		c.NodeCount, _ = strconv.Atoi(nodeCountStr)
	}

	if valWaitStr := os.Getenv("VALIDATION_WAIT"); valWaitStr != "" {
		c.ValidationWait, _ = time.ParseDuration(valWaitStr)
	}

	if err := c.Finalize(); err != nil {
		return nil, err
	}

	return c, nil
}

// Finalize ensures all config fields have valid defaults and derived paths.
func (c *Config) Finalize() error {
	repoRoot, err := repoRoot()
	if err != nil {
		return err
	}
	workspace := repoRoot

	if c.ClusterName == "" {
		c.ClusterName = "kops.k8s.local"
	}

	workDir := filepath.Join(workspace, "clusters", c.ClusterName)

	if c.GCPLocation == "" {
		c.GCPLocation = "us-central1"
	}
	if c.GCPZones == "" {
		c.GCPZones = fmt.Sprintf("%s-b", c.GCPLocation)
	}

	if c.NodeCount <= 0 {
		c.NodeCount = 2
	}

	if c.K8sVersion == "" {
		if v, err := versionFromFile(repoRoot); err == nil {
			c.K8sVersion = v
		} else if v, err := latestK8sVersion(); err == nil {
			c.K8sVersion = v
		}
	}

	if c.KopsBin == "" {
		c.KopsBin = filepath.Join(workspace, "bin", "kops")
	}

	if c.KopsBaseURL == "" {
		if v, err := latestKopsVersion(); err == nil {
			c.KopsBaseURL = v
		}
	}

	if c.TemplateSrc == "" {
		c.TemplateSrc = filepath.Join(repoRoot, "test", "kops-cluster.yaml.template")
	}

	if c.TemplatePath == "" {
		c.TemplatePath = filepath.Join(workDir, "kops-cluster.yaml")
	}

	if c.AdminAccess == "" {
		c.AdminAccess = "0.0.0.0/0"
	}

	if c.ControlPlaneMachineType == "" {
		c.ControlPlaneMachineType = "e2-standard-2"
	}

	if c.NodeMachineType == "" {
		c.NodeMachineType = "e2-standard-2"
	}

	if c.SSHPrivateKey == "" {
		c.SSHPrivateKey = filepath.Join(workDir, "google_compute_engine")
	}
	c.SSHPublicKey = c.SSHPrivateKey + ".pub"

	if c.ValidationWait <= 0 {
		c.ValidationWait = 20 * time.Minute
	}

	return nil
}

// UpdateConfigFromFlags is now a thin wrapper around Finalize.
func UpdateConfigFromFlags(c *Config) error {
	return c.Finalize()
}

func repoRoot() (string, error) {
	pwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(pwd, ".git")); err == nil {
			return pwd, nil
		}
		parent := filepath.Dir(pwd)
		if parent == pwd {
			return "", fmt.Errorf("could not find repo root")
		}
		pwd = parent
	}
}

func versionFromFile(repoRoot string) (string, error) {
	path := filepath.Join(repoRoot, "ginko-test-package-version.env")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func latestK8sVersion() (string, error) {
	return versionFromURL("https://dl.k8s.io/release/stable.txt")
}

func latestKopsVersion() (string, error) {
	return versionFromURL("https://storage.googleapis.com/k8s-staging-kops/kops/releases/markers/master/latest-ci-updown-green.txt")
}

func versionFromURL(url string) (string, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(body)), nil
}
