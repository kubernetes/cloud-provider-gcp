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
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	"cloud.google.com/go/compute/metadata"
	"cloud.google.com/go/iam"
	"cloud.google.com/go/storage"
	"golang.org/x/crypto/ssh"
	"golang.org/x/oauth2/google"
)

// EnsureStateStore ensures the GCS bucket for kOps state exists and has correct settings.
func EnsureStateStore(c *Config) error {
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create storage client: %v", err)
	}
	defer client.Close()

	if c.StateStore == "" {
		if c.GCPProject == "" {
			return fmt.Errorf("GCP_PROJECT must be set if KOPS_STATE_STORE is not provided")
		}
		c.StateStore = fmt.Sprintf("gs://kops-state-%s", c.GCPProject)
	}

	bucketName := strings.TrimPrefix(c.StateStore, "gs://")
	fmt.Printf("Ensuring KOPS_STATE_STORE exists: %s\n", c.StateStore)

	bucket := client.Bucket(bucketName)
	attrs, err := bucket.Attrs(ctx)
	if err != nil {
		if errors.Is(err, storage.ErrBucketNotExist) {
			fmt.Printf("Bucket %s does not exist, creating...\n", c.StateStore)
			if err := bucket.Create(ctx, c.GCPProject, &storage.BucketAttrs{
				Location: c.GCPLocation,
			}); err != nil {
				return fmt.Errorf("failed to create bucket: %v", err)
			}
			attrs, err = bucket.Attrs(ctx)
			if err != nil {
				return fmt.Errorf("failed to get bucket attrs after creation: %v", err)
			}
		} else {
			return fmt.Errorf("failed to check if bucket exists: %v", err)
		}
	}

	// Disable uniform bucket-level access
	if attrs.UniformBucketLevelAccess.Enabled {
		fmt.Printf("Disabling uniform bucket-level access for %s\n", c.StateStore)
		_, err := bucket.Update(ctx, storage.BucketAttrsToUpdate{
			UniformBucketLevelAccess: &storage.UniformBucketLevelAccess{
				Enabled: false,
			},
		})
		if err != nil {
			return fmt.Errorf("failed to disable UBLA: %v", err)
		}
	}

	// Grant storage.admin to the current account
	var sa string
	if metadata.OnGCE() {
		sa, err = metadata.Email("default")
		if err != nil {
			return fmt.Errorf("failed to get current account from metadata: %v", err)
		}
	} else {
		// Fallback to gcloud if not on GCE, as a transition measure or for local dev
		// But the goal is to remove gcloud. Let's try to get it from environment.
		sa = os.Getenv("GCP_ACCOUNT")
		if sa == "" {
			// If we still can't find it, we might have to use gcloud as a last resort
			// or just fail if that's the requirement.
			// The issue says "Replace gcloud ... with client libraries"
			// Let's try to get it from gcloud if we must, but the instruction is to replace it.
			// Actually, if we are using GCP client libraries, the user might have set up ADC.
			// Let's use a helper that tries to be smart.
			sa, err = discoverAccount(ctx)
			if err != nil {
				return fmt.Errorf("failed to discover current account: %v", err)
			}
		}
	}

	fmt.Printf("Granting storage.admin to %s on %s\n", sa, c.StateStore)
	policy, err := bucket.IAM().Policy(ctx)
	if err != nil {
		return fmt.Errorf("failed to get IAM policy: %v", err)
	}

	// Try serviceAccount first, then user (similar to original logic)
	role := iam.RoleName("roles/storage.admin")
	member := "serviceAccount:" + sa
	policy.Add(member, role)

	if err := bucket.IAM().SetPolicy(ctx, policy); err != nil {
		// If it fails, maybe it's a user account
		fmt.Printf("Warning: failed to grant storage.admin to %s as serviceAccount: %v. Retrying as user...\n", sa, err)
		member = "user:" + sa
		policy.Add(member, role)
		if err := bucket.IAM().SetPolicy(ctx, policy); err != nil {
			fmt.Printf("Warning: failed to grant storage.admin to %s as user: %v\n", sa, err)
		}
	}

	return nil
}

func discoverAccount(ctx context.Context) (string, error) {
	if metadata.OnGCE() {
		return metadata.Email("default")
	}

	// Try to get from Application Default Credentials
	creds, err := google.FindDefaultCredentials(ctx)
	if err == nil && len(creds.JSON) > 0 {
		var config struct {
			ClientEmail string `json:"client_email"`
		}
		if err := json.Unmarshal(creds.JSON, &config); err == nil && config.ClientEmail != "" {
			return config.ClientEmail, nil
		}
	}

	return "", fmt.Errorf("could not determine current account without gcloud or being on GCE (tried metadata and ADC)")
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
	if err := generateSSHKey(c.SSHPrivateKey, c.SSHPublicKey); err != nil {
		return fmt.Errorf("failed to generate SSH key: %v", err)
	}

	// Add the key to project metadata. This replaces the part of gcloud compute config-ssh
	// that ensures the key is available for GCE.
	pubKeyBytes, err := os.ReadFile(c.SSHPublicKey)
	if err != nil {
		return fmt.Errorf("failed to read public key: %v", err)
	}

	ctx := context.Background()
	if err := addSSHKeyToProject(ctx, c.GCPProject, string(pubKeyBytes)); err != nil {
		// We log this as a warning because it might fail due to permissions,
		// but kOps might still be able to use the local key.
		fmt.Printf("Warning: failed to add SSH key to project metadata: %v\n", err)
	}

	return nil
}

// CleanSSHKey cleanly removes SSH configuration metadata appended by kOps and deletes the generated keys.
func CleanSSHKey(c *Config) error {
	if c.SSHPrivateKey == "" {
		return nil
	}

	fmt.Printf("Cleaning up SSH configuration and keys...\n")

	// Remove from project metadata
	pubKeyBytes, err := os.ReadFile(c.SSHPublicKey)
	if err == nil {
		ctx := context.Background()
		if err := removeSSHKeyFromProject(ctx, c.GCPProject, string(pubKeyBytes)); err != nil {
			fmt.Printf("Warning: failed to remove SSH key from project metadata: %v\n", err)
		}
	}

	// Remove the actual key files if they exist
	_ = os.Remove(c.SSHPrivateKey)
	_ = os.Remove(c.SSHPublicKey)

	return nil
}

func generateSSHKey(privateKeyPath, publicKeyPath string) error {
	// Create directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(privateKeyPath), 0700); err != nil {
		return err
	}

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}

	// Private key in PEM format
	privBytes := x509.MarshalPKCS1PrivateKey(priv)
	privBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privBytes,
	}
	privFile, err := os.OpenFile(privateKeyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer privFile.Close()
	if err := pem.Encode(privFile, privBlock); err != nil {
		return err
	}

	// Public key in authorized_keys format
	pub, err := ssh.NewPublicKey(&priv.PublicKey)
	if err != nil {
		return err
	}
	pubBytes := ssh.MarshalAuthorizedKey(pub)

	// Add a comment to the public key
	user := os.Getenv("USER")
	if user == "" {
		user = "kops"
	}
	comment := fmt.Sprintf(" %s@kops", user)
	pubWithComment := strings.TrimSpace(string(pubBytes)) + comment + "\n"

	if err := os.WriteFile(publicKeyPath, []byte(pubWithComment), 0644); err != nil {
		return err
	}

	return nil
}

func addSSHKeyToProject(ctx context.Context, project, publicKey string) error {
	client, err := compute.NewProjectsRESTClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	proj, err := client.Get(ctx, &computepb.GetProjectRequest{
		Project: project,
	})
	if err != nil {
		return err
	}

	var sshKeys string
	metadata := proj.CommonInstanceMetadata
	if metadata == nil {
		metadata = &computepb.Metadata{}
	}

	for _, item := range metadata.Items {
		if item.Key != nil && *item.Key == "ssh-keys" {
			if item.Value != nil {
				sshKeys = *item.Value
			}
			break
		}
	}

	// Format: <user>:ssh-rsa <key> <comment>
	user := os.Getenv("USER")
	if user == "" {
		user = "kops"
	}
	newKeyEntry := fmt.Sprintf("%s:%s", user, strings.TrimSpace(publicKey))

	if strings.Contains(sshKeys, newKeyEntry) {
		return nil
	}

	if sshKeys != "" {
		sshKeys += "\n"
	}
	sshKeys += newKeyEntry

	// Update metadata
	found := false
	for i, item := range metadata.Items {
		if item.Key != nil && *item.Key == "ssh-keys" {
			metadata.Items[i].Value = &sshKeys
			found = true
			break
		}
	}
	if !found {
		keyName := "ssh-keys"
		metadata.Items = append(metadata.Items, &computepb.Items{
			Key:   &keyName,
			Value: &sshKeys,
		})
	}

	op, err := client.SetCommonInstanceMetadata(ctx, &computepb.SetCommonInstanceMetadataProjectRequest{
		Project:          project,
		MetadataResource: metadata,
	})
	if err != nil {
		return err
	}

	return op.Wait(ctx)
}

func removeSSHKeyFromProject(ctx context.Context, project, publicKey string) error {
	client, err := compute.NewProjectsRESTClient(ctx)
	if err != nil {
		return err
	}
	defer client.Close()

	proj, err := client.Get(ctx, &computepb.GetProjectRequest{
		Project: project,
	})
	if err != nil {
		return err
	}

	metadata := proj.CommonInstanceMetadata
	if metadata == nil {
		return nil
	}

	found := false
	for i, item := range metadata.Items {
		if item.Key != nil && *item.Key == "ssh-keys" {
			if item.Value == nil {
				continue
			}
			lines := strings.Split(*item.Value, "\n")
			newLines := []string{}
			keyPart := strings.TrimSpace(publicKey)
			for _, line := range lines {
				if !strings.Contains(line, keyPart) {
					newLines = append(newLines, line)
				} else {
					found = true
				}
			}
			if found {
				newValue := strings.Join(newLines, "\n")
				metadata.Items[i].Value = &newValue
			}
			break
		}
	}

	if !found {
		return nil
	}

	op, err := client.SetCommonInstanceMetadata(ctx, &computepb.SetCommonInstanceMetadataProjectRequest{
		Project:          project,
		MetadataResource: metadata,
	})
	if err != nil {
		return err
	}

	return op.Wait(ctx)
}
