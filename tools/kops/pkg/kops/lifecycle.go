package kops

import (
	"fmt"
	"io/ioutil"
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

	args := []string{"kops"}
	args = append(args, "-v=2")
	args = append(args, "--cloud-provider=gce")
	args = append(args, "--cluster-name="+c.ClusterName)
	args = append(args, "--kops-binary-path="+c.KopsBin)
	args = append(args, "--admin-access="+c.AdminAccess)
	args = append(args, "--validation-wait="+c.ValidationWait.String())
	
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

	// The 'up' specific args
	upArgs := []string{"--up"}
	if c.K8sVersion != "" {
		upArgs = append(upArgs, "--kubernetes-version="+c.K8sVersion)
	}
	upArgs = append(upArgs, "--template-path="+c.TemplatePath)

	// Inject local CCM image if requested
	featureFlags := c.KopsFeatureFlags
	if c.ImageRepo != "" && c.ImageTag != "" {
		repoRoot, err := getRepoRoot()
		if err != nil {
			return err
		}
		manifestSrc := filepath.Join(repoRoot, "deploy/packages/default/manifest.yaml")
		data, err := ioutil.ReadFile(manifestSrc)
		if err != nil {
			return fmt.Errorf("failed to read CCM manifest: %v", err)
		}

		localImage := fmt.Sprintf("%s/cloud-controller-manager:%s", c.ImageRepo, c.ImageTag)
		replaced := strings.ReplaceAll(string(data), "k8scloudprovidergcp/cloud-controller-manager:latest", localImage)

		// Save temporary manifest in the cluster workdir
		manifestPath := filepath.Join(filepath.Dir(c.TemplatePath), "cloud-provider-gcp.yaml")
		if err := ioutil.WriteFile(manifestPath, []byte(replaced), 0644); err != nil {
			return fmt.Errorf("failed to write local CCM manifest: %v", err)
		}

		upArgs = append(upArgs, "--create-args=--add="+manifestPath)
		
		// Enable ClusterAddons feature flag
		if featureFlags == "" {
			featureFlags = "ClusterAddons"
		} else if !strings.Contains(featureFlags, "ClusterAddons") {
			featureFlags += ",ClusterAddons"
		}
	}
	
	// Add feature flags to common args
	args = append(args, "--env=KOPS_FEATURE_FLAGS="+featureFlags)

	// Combine all args: kubetest2 kops [common args] [up specific args]
	fullArgs := append(args, upArgs...)

	fmt.Printf("Running kubetest2 %s\n", strings.Join(fullArgs, " "))
	cmd := exec.Command("kubetest2", fullArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	// Ensure critical variables are in the environment for the subprocess
	setEnvIfNotEmpty(cmd, "KOPS_STATE_STORE", c.StateStore)
	setEnvIfNotEmpty(cmd, "GCP_PROJECT", c.GCPProject)
	setEnvIfNotEmpty(cmd, "CLUSTER_NAME", c.ClusterName)
	setEnvIfNotEmpty(cmd, "K8S_VERSION", c.K8sVersion)
	setEnvIfNotEmpty(cmd, "KOPS_BASE_URL", c.KopsBaseURL)

	return cmd.Run()
}

// Down tears down the cluster.
func Down(c *Config) error {
	args := []string{"kops"}
	args = append(args, "-v=2")
	args = append(args, "--cloud-provider=gce")
	args = append(args, "--cluster-name="+c.ClusterName)
	args = append(args, "--kops-binary-path="+c.KopsBin)
	args = append(args, "--admin-access="+c.AdminAccess)
	
	if c.GCPProject != "" {
		args = append(args, "--gcp-project="+c.GCPProject)
	}

	if c.SSHPrivateKey != "" {
		args = append(args, "--ssh-private-key="+c.SSHPrivateKey)
		sshUser := os.Getenv("KUBE_SSH_USER")
		if sshUser == "" {
			sshUser = os.Getenv("USER")
		}
		if sshUser != "" {
			args = append(args, "--ssh-user="+sshUser)
		}
	}
	
	// The 'down' specific args
	downArgs := append(args, "--down")

	fmt.Printf("Running kubetest2 %s\n", strings.Join(downArgs, " "))
	cmd := exec.Command("kubetest2", downArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	setEnvIfNotEmpty(cmd, "KOPS_STATE_STORE", c.StateStore)
	setEnvIfNotEmpty(cmd, "GCP_PROJECT", c.GCPProject)
	setEnvIfNotEmpty(cmd, "KOPS_BASE_URL", c.KopsBaseURL)

	return cmd.Run()
}

func setEnvIfNotEmpty(cmd *exec.Cmd, key, value string) {
	if value != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}
}
