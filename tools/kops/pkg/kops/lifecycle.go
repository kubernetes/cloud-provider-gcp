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
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Up provisions the cluster.
func Up(c *Config) error {
	if err := EnsureStateStore(c); err != nil {
		return err
	}
	if err := EnsureSSHKey(c); err != nil {
		return err
	}

	args := commonArgs(c)

	// Inject local CCM image if requested
	featureFlags := c.KopsFeatureFlags
	if c.ImageRepo != "" && c.ImageTag != "" {
		manifestPath, err := prepareLocalCCMManifest(c)
		if err != nil {
			return err
		}

		args = append(args, "--create-args=--add="+manifestPath)

		// Enable ClusterAddons feature flag
		if featureFlags == "" {
			featureFlags = "ClusterAddons"
		} else if !strings.Contains(featureFlags, "ClusterAddons") {
			featureFlags += ",ClusterAddons"
		}
	}

	args = append(args, "--env=KOPS_FEATURE_FLAGS="+featureFlags)
	args = append(args, "--up")
	args = append(args, "--template-path="+c.TemplatePath)
	if c.K8sVersion != "" {
		args = append(args, "--kubernetes-version="+c.K8sVersion)
	}

	return runKubetest2(c, args)
}

// Down tears down the cluster.
func Down(c *Config) error {
	args := commonArgs(c)
	args = append(args, "--down")
	return runKubetest2(c, args)
}

func commonArgs(c *Config) []string {
	args := []string{
		"kops",
		"-v=2",
		"--cloud-provider=gce",
		"--cluster-name=" + c.ClusterName,
		"--kops-binary-path=" + c.KopsBin,
		"--admin-access=" + c.AdminAccess,
		"--validation-wait=" + c.ValidationWait.String(),
	}

	if c.GCPProject != "" {
		args = append(args, "--gcp-project="+c.GCPProject)
	}

	if c.SSHPrivateKey != "" {
		args = append(args, "--ssh-private-key="+c.SSHPrivateKey)
		args = append(args, "--ssh-public-key="+c.SSHPublicKey)
		sshUser := os.Getenv("KUBE_SSH_USER")
		if sshUser == "" {
			sshUser = os.Getenv("USER")
		}
		if sshUser != "" {
			args = append(args, "--ssh-user="+sshUser)
		}
	}

	if c.GoogleAppCredentials != "" {
		args = append(args, "--env=GOOGLE_APPLICATION_CREDENTIALS="+c.GoogleAppCredentials)
	}

	return args
}

func runKubetest2(c *Config, args []string) error {
	fmt.Printf("Running kubetest2 %s\n", strings.Join(args, " "))
	cmd := exec.Command("kubetest2", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Create a clean environment based on current process
	env := os.Environ()
	newEnv := []string{}
	for _, e := range env {
		// Filter out KOPS_CLUSTER_NAME and CLUSTER_NAME to prevent ambiguity
		if !strings.HasPrefix(e, "KOPS_CLUSTER_NAME=") && !strings.HasPrefix(e, "CLUSTER_NAME=") && !strings.HasPrefix(e, "PATH=") {
			newEnv = append(newEnv, e)
		}
	}

	// Add local bin to PATH so kubetest2 can find its plugins
	repoRoot, _ := repoRoot()
	localBin := filepath.Join(repoRoot, "bin")
	path := os.Getenv("PATH")
	newEnv = append(newEnv, fmt.Sprintf("PATH=%s:%s", localBin, path))

	cmd.Env = newEnv

	// Ensure critical variables are in the environment for the subprocess
	setEnvIfNotEmpty(cmd, "KOPS_STATE_STORE", c.StateStore)
	setEnvIfNotEmpty(cmd, "GCP_PROJECT", c.GCPProject)
	setEnvIfNotEmpty(cmd, "K8S_VERSION", c.K8sVersion)
	setEnvIfNotEmpty(cmd, "KOPS_BASE_URL", c.KopsBaseURL)

	return cmd.Run()
}

func prepareLocalCCMManifest(c *Config) (string, error) {
	repoRoot, err := repoRoot()
	if err != nil {
		return "", err
	}
	manifestSrc := filepath.Join(repoRoot, "deploy/packages/default/manifest.yaml")
	data, err := os.ReadFile(manifestSrc)
	if err != nil {
		return "", fmt.Errorf("failed to read CCM manifest: %v", err)
	}

	localImage := fmt.Sprintf("%s/cloud-controller-manager:%s", c.ImageRepo, c.ImageTag)
	replaced := strings.ReplaceAll(string(data), "k8scloudprovidergcp/cloud-controller-manager:latest", localImage)

	// Save temporary manifest in the cluster workdir
	manifestPath := filepath.Join(filepath.Dir(c.TemplatePath), "cloud-provider-gcp.yaml")
	if err := os.WriteFile(manifestPath, []byte(replaced), 0644); err != nil {
		return "", fmt.Errorf("failed to write local CCM manifest: %v", err)
	}

	return manifestPath, nil
}

func setEnvIfNotEmpty(cmd *exec.Cmd, key, value string) {
	if value != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}
}
