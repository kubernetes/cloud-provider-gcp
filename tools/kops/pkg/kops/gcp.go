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
		// Assume it doesn't exist, try to create it
		fmt.Printf("Bucket %s does not exist, creating...\n", c.StateStore)
		mbCmd := exec.Command("gsutil", "mb", "-p", c.GCPProject, "-l", c.GCPLocation, c.StateStore)
		mbCmd.Stdout = os.Stdout
		mbCmd.Stderr = os.Stderr
		if err := mbCmd.Run(); err != nil {
			return fmt.Errorf("failed to create bucket: %v", err)
		}
	}

	// Disable uniform bucket-level access
	ublaCmd := exec.Command("gsutil", "ubla", "set", "off", c.StateStore)
	ublaCmd.Stdout = os.Stdout
	ublaCmd.Stderr = os.Stderr
	if err := ublaCmd.Run(); err != nil {
		return fmt.Errorf("failed to disable UBLA: %v", err)
	}

	// Grant storage.admin to the current account
	// SA=$(gcloud config list --format 'value(core.account)')
	saCmd := exec.Command("gcloud", "config", "list", "--format", "value(core.account)")
	saBytes, err := saCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get current account: %v", err)
	}
	sa := strings.TrimSpace(string(saBytes))

	iamCmd := exec.Command("gsutil", "iam", "ch", fmt.Sprintf("serviceAccount:%s:admin", sa), c.StateStore)
	iamCmd.Stdout = os.Stdout
	iamCmd.Stderr = os.Stderr
	if err := iamCmd.Run(); err != nil {
		// This might fail if the account is not a service account (e.g. user account)
		// The bash script assumes serviceAccount:
		fmt.Printf("Warning: failed to grant storage.admin to %s: %v. Retrying with user account...\n", sa, err)
		iamUserCmd := exec.Command("gsutil", "iam", "ch", fmt.Sprintf("user:%s:admin", sa), c.StateStore)
		iamUserCmd.Stdout = os.Stdout
		iamUserCmd.Stderr = os.Stderr
		if err := iamUserCmd.Run(); err != nil {
			fmt.Printf("Warning: failed to grant storage.admin to user %s: %v\n", sa, err)
		}
	}

	return nil
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
