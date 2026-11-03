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
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	"cloud.google.com/go/compute/metadata"
	"cloud.google.com/go/iam"
	"cloud.google.com/go/storage"
	"golang.org/x/crypto/ssh"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/googleapi"
)

// EnsureStateStore ensures the GCS bucket for kOps state exists and has correct settings.
func EnsureStateStore(c *Config) error {
	if c.StateStore == "" {
		if c.GCPProject == "" {
			return fmt.Errorf("GCP_PROJECT must be set if KOPS_STATE_STORE is not provided")
		}
		c.StateStore = fmt.Sprintf("gs://kops-state-%s", c.GCPProject)
	}

	bucketName := strings.TrimSuffix(strings.TrimPrefix(c.StateStore, "gs://"), "/")
	if bucketName == "" {
		return fmt.Errorf("invalid state store bucket name: %q", c.StateStore)
	}

	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create storage client: %w", err)
	}
	defer client.Close()

	fmt.Printf("Ensuring KOPS_STATE_STORE exists: %s\n", c.StateStore)

	bucket := client.Bucket(bucketName)
	attrs, err := bucket.Attrs(ctx)
	if err != nil {
		if errors.Is(err, storage.ErrBucketNotExist) {
			if c.GCPProject == "" {
				return fmt.Errorf("GCP_PROJECT must be set to create bucket %s", c.StateStore)
			}
			fmt.Printf("Bucket %s does not exist, creating...\n", c.StateStore)
			err := runWithRetry(func() error {
				createErr := bucket.Create(ctx, c.GCPProject, &storage.BucketAttrs{
					Location: c.GCPLocation,
				})
				var apiErr *googleapi.Error
				if errors.As(createErr, &apiErr) && apiErr.Code == http.StatusConflict {
					return nil
				}
				return createErr
			}, "creating state store bucket", 3, 2*time.Second)
			if err != nil {
				return fmt.Errorf("failed to create bucket: %w", err)
			}
		} else {
			return fmt.Errorf("failed to check if bucket exists: %w", err)
		}
	}

	// Poll until GCS bucket creation/propagation is complete and ready for operations
	if err := waitForBucketReadiness(ctx, bucket, c.StateStore, 15, 2*time.Second); err != nil {
		return fmt.Errorf("state store bucket is not ready: %w", err)
	}

	if attrs == nil {
		attrs, err = bucket.Attrs(ctx)
		if err != nil {
			return fmt.Errorf("failed to get bucket attrs after creation: %w", err)
		}
	}

	// Disable uniform bucket-level access with retries
	if attrs.UniformBucketLevelAccess.Enabled {
		fmt.Printf("Disabling uniform bucket-level access for %s\n", c.StateStore)
		err := runWithRetry(func() error {
			_, updateErr := bucket.Update(ctx, storage.BucketAttrsToUpdate{
				UniformBucketLevelAccess: &storage.UniformBucketLevelAccess{
					Enabled: false,
				},
			})
			return updateErr
		}, "disabling UBLA", 5, 2*time.Second)
		if err != nil {
			return fmt.Errorf("failed to disable UBLA: %w", err)
		}
	}

	// Grant storage.admin to the current account
	sa := strings.TrimSpace(os.Getenv("GCP_ACCOUNT"))
	if sa == "" {
		sa, err = discoverAccount(ctx)
		if err != nil {
			return fmt.Errorf("failed to discover current account: %w", err)
		}
	}

	fmt.Printf("Granting storage.admin to %s on %s\n", sa, c.StateStore)
	role := iam.RoleName("roles/storage.admin")
	err = runWithRetry(func() error {
		policy, policyErr := bucket.IAM().Policy(ctx)
		if policyErr != nil {
			return fmt.Errorf("failed to get IAM policy: %w", policyErr)
		}
		member := "serviceAccount:" + sa
		if !policy.HasRole(member, role) {
			policy.Add(member, role)
			if setErr := bucket.IAM().SetPolicy(ctx, policy); setErr != nil {
				return setErr
			}
		}
		return nil
	}, "granting serviceAccount admin IAM", 3, 2*time.Second)

	if err != nil {
		fmt.Printf("Warning: failed to grant storage.admin to %s as serviceAccount: %v. Retrying as user...\n", sa, err)
		errUser := runWithRetry(func() error {
			policy, policyErr := bucket.IAM().Policy(ctx)
			if policyErr != nil {
				return fmt.Errorf("failed to get IAM policy: %w", policyErr)
			}
			member := "user:" + sa
			if !policy.HasRole(member, role) {
				policy.Add(member, role)
				if setErr := bucket.IAM().SetPolicy(ctx, policy); setErr != nil {
					return setErr
				}
			}
			return nil
		}, "granting user admin IAM", 3, 2*time.Second)
		if errUser != nil {
			fmt.Printf("Warning: failed to grant storage.admin to %s as user: %v\n", sa, errUser)
		}
	}

	return nil
}

func runWithRetry(fn func() error, description string, maxRetries int, delay time.Duration) error {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		if err := fn(); err == nil {
			return nil
		} else {
			lastErr = err
			fmt.Printf("Retrying %s (attempt %d/%d) after error: %v\n", description, i+1, maxRetries, err)
			if i+1 < maxRetries {
				time.Sleep(delay)
			}
		}
	}
	return fmt.Errorf("%s failed after %d attempts: %w", description, maxRetries, lastErr)
}

// waitForBucketReadiness polls the state store bucket until it is verified to be accessible and writable.
func waitForBucketReadiness(ctx context.Context, bucket *storage.BucketHandle, stateStore string, maxRetries int, delay time.Duration) error {
	fmt.Printf("Waiting for KOPS_STATE_STORE readiness: %s...\n", stateStore)
	probeObjName := fmt.Sprintf(".probe_%d", time.Now().UnixNano())

	var lastErr error
	for i := 0; i < maxRetries; i++ {
		obj := bucket.Object(probeObjName)
		w := obj.NewWriter(ctx)
		_, writeErr := w.Write([]byte("probe"))
		closeErr := w.Close()
		if writeErr == nil && closeErr == nil {
			_ = obj.Delete(ctx)
			fmt.Printf("KOPS_STATE_STORE %s is ready and writable.\n", stateStore)
			return nil
		}
		if writeErr != nil {
			lastErr = writeErr
		} else {
			lastErr = closeErr
		}
		fmt.Printf("Waiting for bucket readiness (%s) (attempt %d/%d): %v\n", stateStore, i+1, maxRetries, lastErr)
		if i+1 < maxRetries {
			time.Sleep(delay)
		}
	}
	return fmt.Errorf("bucket %s did not become ready after %d retries: %w", stateStore, maxRetries, lastErr)
}

func discoverAccount(ctx context.Context) (string, error) {
	// 1. Try to get from Application Default Credentials (ADC) file first.
	// This ensures we get the real service account if GOOGLE_APPLICATION_CREDENTIALS is set,
	// even when running on GCE (like GKE pods in CI/CD).
	creds, err := google.FindDefaultCredentials(ctx)
	if err == nil && len(creds.JSON) > 0 {
		if email := extractEmailFromCredentialsJSON(creds.JSON); isValidEmail(email) {
			return email, nil
		}
	}

	// 2. Fallback to parsing GOOGLE_APPLICATION_CREDENTIALS environment variable directly
	// before consulting GCE metadata so explicit credential files always take precedence.
	if credsFile := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"); credsFile != "" {
		if data, err := os.ReadFile(credsFile); err == nil {
			if email := extractEmailFromCredentialsJSON(data); isValidEmail(email) {
				return email, nil
			}
		}
	}

	// 3. Try to get from GCE metadata server.
	if metadata.OnGCE() {
		email, err := metadata.Email("default")
		if err == nil && isValidEmail(email) {
			return strings.TrimSpace(email), nil
		}
	}

	return "", fmt.Errorf("could not determine current account (tried ADC, GOOGLE_APPLICATION_CREDENTIALS, and metadata)")
}

func extractEmailFromCredentialsJSON(data []byte) string {
	var config struct {
		ClientEmail string `json:"client_email"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return ""
	}
	return strings.TrimSpace(config.ClientEmail)
}

func isValidEmail(email string) bool {
	email = strings.TrimSpace(email)
	at := strings.IndexByte(email, '@')
	return at > 0 && at < len(email)-1 && !strings.ContainsAny(email, " \t\r\n")
}

// EnsureSSHKey ensures that an SSH key exists for kOps.
func EnsureSSHKey(c *Config) error {
	if c.SSHPrivateKey == "" {
		return fmt.Errorf("SSHPrivateKey must be set in config")
	}
	if c.SSHPublicKey == "" {
		c.SSHPublicKey = c.SSHPrivateKey + ".pub"
	}

	if _, err := os.Stat(c.SSHPrivateKey); err == nil {
		if _, pubErr := os.Stat(c.SSHPublicKey); pubErr == nil {
			fmt.Printf("SSH key already exists at %s\n", c.SSHPrivateKey)
			return nil
		}
	}

	fmt.Printf("SSH key %s not found, creating one...\n", c.SSHPrivateKey)
	if err := generateSSHKey(c.SSHPrivateKey, c.SSHPublicKey); err != nil {
		return fmt.Errorf("failed to generate SSH key: %w", err)
	}

	// Add the key to project metadata. This replaces the part of gcloud compute config-ssh
	// that ensures the key is available for GCE.
	pubKeyBytes, err := os.ReadFile(c.SSHPublicKey)
	if err != nil {
		return fmt.Errorf("failed to read public key: %w", err)
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
	if c.SSHPublicKey == "" {
		c.SSHPublicKey = c.SSHPrivateKey + ".pub"
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

func sshUsername() string {
	if user := os.Getenv("KUBE_SSH_USER"); user != "" {
		return user
	}
	if user := os.Getenv("USER"); user != "" {
		return user
	}
	return "kops"
}

func generateSSHKey(privateKeyPath, publicKeyPath string) error {
	if err := os.MkdirAll(filepath.Dir(privateKeyPath), 0700); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(publicKeyPath), 0700); err != nil {
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
	if err := os.WriteFile(privateKeyPath, pem.EncodeToMemory(privBlock), 0600); err != nil {
		return err
	}

	// Public key in authorized_keys format
	pub, err := ssh.NewPublicKey(&priv.PublicKey)
	if err != nil {
		_ = os.Remove(privateKeyPath)
		return err
	}
	pubBytes := ssh.MarshalAuthorizedKey(pub)

	// Add a comment to the public key
	comment := fmt.Sprintf(" %s@kops", sshUsername())
	pubWithComment := strings.TrimSpace(string(pubBytes)) + comment + "\n"

	if err := os.WriteFile(publicKeyPath, []byte(pubWithComment), 0644); err != nil {
		_ = os.Remove(privateKeyPath)
		return err
	}

	return nil
}

func addSSHKeyToProject(ctx context.Context, project, publicKey string) error {
	if project == "" {
		return fmt.Errorf("GCP project must not be empty")
	}
	if strings.TrimSpace(publicKey) == "" {
		return fmt.Errorf("public key must not be empty")
	}

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

	md, updated := addSSHKeyToMetadata(proj.CommonInstanceMetadata, sshUsername(), publicKey)
	if !updated {
		return nil
	}

	op, err := client.SetCommonInstanceMetadata(ctx, &computepb.SetCommonInstanceMetadataProjectRequest{
		Project:          project,
		MetadataResource: md,
	})
	if err != nil {
		return err
	}

	return op.Wait(ctx)
}

func addSSHKeyToMetadata(md *computepb.Metadata, user, publicKey string) (*computepb.Metadata, bool) {
	trimmedKey := strings.TrimSpace(publicKey)
	if trimmedKey == "" {
		return md, false
	}
	if user == "" {
		user = "kops"
	}
	if md == nil {
		md = &computepb.Metadata{}
	}

	var sshKeys string
	for _, item := range md.Items {
		if item != nil && item.Key != nil && *item.Key == "ssh-keys" {
			if item.Value != nil {
				sshKeys = *item.Value
			}
			break
		}
	}

	newKeyEntry := fmt.Sprintf("%s:%s", user, trimmedKey)
	for _, line := range strings.Split(sshKeys, "\n") {
		if strings.TrimSpace(line) == newKeyEntry {
			return md, false
		}
	}

	if sshKeys != "" {
		sshKeys += "\n"
	}
	sshKeys += newKeyEntry

	found := false
	for i, item := range md.Items {
		if item != nil && item.Key != nil && *item.Key == "ssh-keys" {
			md.Items[i].Value = &sshKeys
			found = true
			break
		}
	}
	if !found {
		keyName := "ssh-keys"
		md.Items = append(md.Items, &computepb.Items{
			Key:   &keyName,
			Value: &sshKeys,
		})
	}

	return md, true
}

func removeSSHKeyFromProject(ctx context.Context, project, publicKey string) error {
	if project == "" {
		return fmt.Errorf("GCP project must not be empty")
	}
	if strings.TrimSpace(publicKey) == "" {
		return nil
	}

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

	if !removeSSHKeyFromMetadata(proj.CommonInstanceMetadata, publicKey) {
		return nil
	}

	op, err := client.SetCommonInstanceMetadata(ctx, &computepb.SetCommonInstanceMetadataProjectRequest{
		Project:          project,
		MetadataResource: proj.CommonInstanceMetadata,
	})
	if err != nil {
		return err
	}

	return op.Wait(ctx)
}

func removeSSHKeyFromMetadata(md *computepb.Metadata, publicKey string) bool {
	if md == nil {
		return false
	}
	keyPart := strings.TrimSpace(publicKey)
	if keyPart == "" {
		return false
	}

	for i, item := range md.Items {
		if item != nil && item.Key != nil && *item.Key == "ssh-keys" {
			if item.Value == nil {
				return false
			}
			lines := strings.Split(*item.Value, "\n")
			newLines := make([]string, 0, len(lines))
			found := false
			for _, line := range lines {
				if strings.Contains(line, keyPart) {
					found = true
				} else {
					newLines = append(newLines, line)
				}
			}
			if found {
				newValue := strings.Join(newLines, "\n")
				md.Items[i].Value = &newValue
				return true
			}
			return false
		}
	}

	return false
}
