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
	"strings"
	"time"
)

// EnsureStateStore ensures the GCS bucket for kOps state exists and has correct settings.
func EnsureStateStore(c *Config) error {
	if c.StateStore == "" {
		if c.GCPProject == "" {
			return fmt.Errorf("GCP_PROJECT must be set if KOPS_STATE_STORE is not provided")
		}
		c.StateStore = fmt.Sprintf("gs://kops-state-%s", c.GCPProject)
	}

	fmt.Printf("Ensuring KOPS_STATE_STORE exists: %s\n", c.StateStore)

	// Check if bucket exists
	lsCmd := exec.Command("gsutil", "ls", "-p", c.GCPProject, c.StateStore)
	if err := lsCmd.Run(); err != nil {
		// Assume it doesn't exist, try to create it with retries
		fmt.Printf("Bucket %s does not exist, creating...\n", c.StateStore)
		err := runWithRetry(func() *exec.Cmd {
			return exec.Command("gsutil", "mb", "-p", c.GCPProject, "-l", c.GCPLocation, c.StateStore)
		}, "creating state store bucket", 3, 2*time.Second)
		if err != nil {
			return fmt.Errorf("failed to create bucket: %v", err)
		}
	}

	// Poll until GCS bucket creation/propagation is complete and ready for operations
	if err := waitForBucketReadiness(c.StateStore, 15, 2*time.Second); err != nil {
		return fmt.Errorf("state store bucket is not ready: %v", err)
	}

	// Disable uniform bucket-level access with retries
	err := runWithRetry(func() *exec.Cmd {
		return exec.Command("gsutil", "ubla", "set", "off", c.StateStore)
	}, "disabling UBLA", 5, 2*time.Second)
	if err != nil {
		return fmt.Errorf("failed to disable UBLA: %v", err)
	}

	// Grant storage.admin to the current account
	saCmd := exec.Command("gcloud", "config", "list", "--format", "value(core.account)")
	saBytes, err := saCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get current account: %v", err)
	}
	sa := strings.TrimSpace(string(saBytes))

	err = runWithRetry(func() *exec.Cmd {
		return exec.Command("gsutil", "iam", "ch", fmt.Sprintf("serviceAccount:%s:admin", sa), c.StateStore)
	}, "granting serviceAccount admin IAM", 3, 2*time.Second)

	if err != nil {
		fmt.Printf("Warning: failed to grant storage.admin to %s: %v. Retrying with user account...\n", sa, err)
		errUser := runWithRetry(func() *exec.Cmd {
			return exec.Command("gsutil", "iam", "ch", fmt.Sprintf("user:%s:admin", sa), c.StateStore)
		}, "granting user admin IAM", 3, 2*time.Second)
		if errUser != nil {
			fmt.Printf("Warning: failed to grant storage.admin to user %s: %v\n", sa, errUser)
		}
	}

	return nil
}

func runWithRetry(cmdFunc func() *exec.Cmd, description string, maxRetries int, delay time.Duration) error {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		cmd := cmdFunc()
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err == nil {
			return nil
		} else {
			lastErr = err
			fmt.Printf("Retrying %s (attempt %d/%d) after error: %v\n", description, i+1, maxRetries, err)
			time.Sleep(delay)
		}
	}
	return fmt.Errorf("%s failed after %d attempts: %v", description, maxRetries, lastErr)
}

// waitForBucketReadiness polls the state store bucket until it is verified to be accessible and writable.
func waitForBucketReadiness(stateStore string, maxRetries int, delay time.Duration) error {
	fmt.Printf("Waiting for KOPS_STATE_STORE readiness: %s...\n", stateStore)
	probeFile := fmt.Sprintf("%s/.probe_%d", strings.TrimSuffix(stateStore, "/"), time.Now().UnixNano())

	var lastErr error
	for i := 0; i < maxRetries; i++ {
		// Try writing a small probe object to the bucket to verify object creation works
		cpCmd := exec.Command("gsutil", "cp", "-", probeFile)
		cpCmd.Stdin = strings.NewReader("probe")
		if err := cpCmd.Run(); err == nil {
			// Probe succeeded; clean up the probe file
			rmCmd := exec.Command("gsutil", "rm", probeFile)
			_ = rmCmd.Run()
			fmt.Printf("KOPS_STATE_STORE %s is ready and writable.\n", stateStore)
			return nil
		} else {
			lastErr = err
			fmt.Printf("Waiting for bucket readiness (%s) (attempt %d/%d): %v\n", stateStore, i+1, maxRetries, err)
			time.Sleep(delay)
		}
	}
	return fmt.Errorf("bucket %s did not become ready after %d retries: %v", stateStore, maxRetries, lastErr)
}

// EnsureSSHKey ensures that an SSH key exists for kOps.
func EnsureSSHKey(c *Config) error {
	if c.SSHPrivateKey == "" {
		return fmt.Errorf("SSHPrivateKey must be set in config")
	}

	if _, err := os.Stat(c.SSHPrivateKey); err == nil {
		fmt.Printf("SSH key already exists at %s\n", c.SSHPrivateKey)
		return nil
	}

	fmt.Printf("SSH key %s not found, creating one...\n", c.SSHPrivateKey)
	// gcloud compute --project="${GCP_PROJECT}" config-ssh --ssh-key-file="${SSH_PRIVATE_KEY}"
	cmd := exec.Command("gcloud", "compute", "--project="+c.GCPProject, "config-ssh", "--ssh-key-file="+c.SSHPrivateKey)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create SSH key: %v", err)
	}

	return nil
}

// CleanSSHKey cleanly removes SSH configuration metadata appended by kOps and deletes the generated keys.
func CleanSSHKey(c *Config) error {
	if c.SSHPrivateKey == "" {
		return nil
	}

	fmt.Printf("Cleaning up SSH configuration and keys...\n")
	cmd := exec.Command("gcloud", "compute", "--project="+c.GCPProject, "config-ssh", "--remove")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("Warning: failed to cleanly remove gcloud ssh configurations: %v\n", err)
	}

	// Remove the actual key files if they exist
	_ = os.Remove(c.SSHPrivateKey)
	_ = os.Remove(c.SSHPublicKey)

	return nil
}
