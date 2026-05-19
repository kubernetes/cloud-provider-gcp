/*
Copyright 2014 The Kubernetes Authors.

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

package gcpcredential

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"k8s.io/cloud-provider-gcp/pkg/credentialconfig"
	"k8s.io/klog/v2"
)

const (
	metadataURL        = "http://metadata.google.internal./computeMetadata/v1/"
	metadataAttributes = metadataURL + "instance/attributes/"
	// DockerConfigKey is the URL of the dockercfg metadata key used by DockerConfigKeyProvider.
	DockerConfigKey = metadataAttributes + "google-dockercfg"
	// DockerConfigURLKey is the URL of the dockercfg metadata key used by DockerConfigURLKeyProvider.
	DockerConfigURLKey = metadataAttributes + "google-dockercfg-url"
	serviceAccounts    = metadataURL + "instance/service-accounts/"
	metadataScopes     = serviceAccounts + "default/scopes"
	metadataToken      = serviceAccounts + "default/token"
	metadataEmail      = serviceAccounts + "default/email"
	// StorageScopePrefix is the prefix checked by ContainerRegistryProvider.Enabled.
	StorageScopePrefix       = "https://www.googleapis.com/auth/devstorage"
	cloudPlatformScopePrefix = "https://www.googleapis.com/auth/cloud-platform"
	defaultServiceAccount    = "default/"
)

// GCEProductNameFile is the product file path that contains the cloud service name.
// This is a variable instead of a const to enable testing.
var GCEProductNameFile = "/sys/class/dmi/id/product_name"

// For these urls, the parts of the host name can be glob, for example '*.gcr.io" will match
// "foo.gcr.io" and "bar.gcr.io".
var containerRegistryUrls = []string{"container.cloud.google.com", "gcr.io", "*.gcr.io", "*.pkg.dev"}

var metadataHeader = &http.Header{
	"Metadata-Flavor": []string{"Google"},
}

// MetadataProvider is a DockerConfigProvider that reads its configuration from Google
// Compute Engine metadata.
type MetadataProvider struct {
	Client *http.Client
}

// DockerConfigKeyProvider is a DockerConfigProvider that reads its configuration from a specific
// Google Compute Engine metadata key: 'google-dockercfg'.
type DockerConfigKeyProvider struct {
	MetadataProvider
}

// DockerConfigURLKeyProvider is a DockerConfigProvider that reads its configuration from a URL read from
// a specific Google Compute Engine metadata key: 'google-dockercfg-url'.
type DockerConfigURLKeyProvider struct {
	MetadataProvider
}

// ContainerRegistryProvider is a DockerConfigProvider that provides a dockercfg with:
//
//	Username: "_token"
//	Password: "{access token from metadata}"
type ContainerRegistryProvider struct {
	MetadataProvider
	UseRegistryFromImage bool

	// Workload Identity context passed by Kubelet via GetResponse
	ServiceAccountToken       string
	ServiceAccountAnnotations map[string]string
}

// Returns true if it finds a local GCE VM.
// Looks at a product file that is an undocumented API.
func onGCEVM() bool {
	var name string

	if runtime.GOOS == "windows" {
		data, err := exec.Command("wmic", "computersystem", "get", "model").Output()
		if err != nil {
			return false
		}
		fields := strings.Split(strings.TrimSpace(string(data)), "\r\n")
		if len(fields) != 2 {
			klog.V(2).Infof("Received unexpected value retrieving system model: %q", string(data))
			return false
		}
		name = fields[1]
	} else {
		data, err := os.ReadFile(GCEProductNameFile)
		if err != nil {
			klog.V(2).Infof("Error while reading product_name: %v", err)
			return false
		}
		name = strings.TrimSpace(string(data))
	}
	return name == "Google" || name == "Google Compute Engine"
}

// Enabled implements DockerConfigProvider for all of the Google implementations.
func (g *MetadataProvider) Enabled() bool {
	return onGCEVM()
}

// Provide implements DockerConfigProvider
func (g *DockerConfigKeyProvider) Provide(image string) credentialconfig.DockerConfig {
	// Read the contents of the google-dockercfg metadata key and
	// parse them as an alternate .dockercfg
	if cfg, err := credentialconfig.ReadDockerConfigFileFromURL(DockerConfigKey, g.Client, metadataHeader); err != nil {
		klog.Errorf("while reading 'google-dockercfg' metadata: %v", err)
	} else {
		return cfg
	}

	return credentialconfig.DockerConfig{}
}

// Provide implements DockerConfigProvider
func (g *DockerConfigURLKeyProvider) Provide(image string) credentialconfig.DockerConfig {
	// Read the contents of the google-dockercfg-url key and load a .dockercfg from there
	if url, err := credentialconfig.ReadURL(DockerConfigURLKey, g.Client, metadataHeader); err != nil {
		klog.Errorf("while reading 'google-dockercfg-url' metadata: %v", err)
	} else {
		if strings.HasPrefix(string(url), "http") {
			if cfg, err := credentialconfig.ReadDockerConfigFileFromURL(string(url), g.Client, nil); err != nil {
				klog.Errorf("while reading 'google-dockercfg-url'-specified url: %s, %v", string(url), err)
			} else {
				return cfg
			}
		} else {
			// TODO(mattmoor): support reading alternate scheme URLs (e.g. gs:// or s3://)
			klog.Errorf("Unsupported URL scheme: %s", string(url))
		}
	}

	return credentialconfig.DockerConfig{}
}

// runWithBackoff runs input function `f` with an exponential backoff.
// Note that this method can block indefinitely.
func runWithBackoff(f func() ([]byte, error)) []byte {
	var backoff = 100 * time.Millisecond
	const maxBackoff = time.Minute
	for {
		value, err := f()
		if err == nil {
			return value
		}
		time.Sleep(backoff)
		backoff = backoff * 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

// Enabled implements a special metadata-based check, which verifies the
// storage scope is available on the GCE VM.
// If running on a GCE VM, check if 'default' service account exists.
// If it does not exist, assume that registry is not enabled.
// If default service account exists, check if relevant scopes exist in the default service account.
// The metadata service can become temporarily inaccesible. Hence all requests to the metadata
// service will be retried until the metadata server returns a `200`.
// It is expected that "http://metadata.google.internal./computeMetadata/v1/instance/service-accounts/" will return a `200`
// and "http://metadata.google.internal./computeMetadata/v1/instance/service-accounts/default/scopes" will also return `200`.
// More information on metadata service can be found here - https://cloud.google.com/compute/docs/storing-retrieving-metadata
func (g *ContainerRegistryProvider) Enabled() bool {
	// Given that we are on GCE, we should keep retrying until the metadata server responds.
	value := runWithBackoff(func() ([]byte, error) {
		value, err := credentialconfig.ReadURL(serviceAccounts, g.Client, metadataHeader)
		if err != nil {
			klog.V(2).Infof("Failed to Get service accounts from gce metadata server: %v", err)
		}
		return value, err
	})
	// We expect the service account to return a list of account directories separated by newlines, e.g.,
	//   sv-account-name1/
	//   sv-account-name2/
	// ref: https://cloud.google.com/compute/docs/storing-retrieving-metadata
	defaultServiceAccountExists := false
	for _, sa := range strings.Split(string(value), "\n") {
		if strings.TrimSpace(sa) == defaultServiceAccount {
			defaultServiceAccountExists = true
			break
		}
	}
	if !defaultServiceAccountExists {
		klog.V(2).Infof("'default' service account does not exist. Found following service accounts: %q", string(value))
		return false
	}
	url := metadataScopes + "?alt=json"
	value = runWithBackoff(func() ([]byte, error) {
		value, err := credentialconfig.ReadURL(url, g.Client, metadataHeader)
		if err != nil {
			klog.V(2).Infof("Failed to Get scopes in default service account from gce metadata server: %v", err)
		}
		return value, err
	})
	var scopes []string
	if err := json.Unmarshal(value, &scopes); err != nil {
		klog.Errorf("Failed to unmarshal scopes: %v", err)
		return false
	}
	for _, v := range scopes {
		// cloudPlatformScope implies storage scope.
		if strings.HasPrefix(v, StorageScopePrefix) || strings.HasPrefix(v, cloudPlatformScopePrefix) {
			return true
		}
	}
	klog.Warningf("Google container registry is disabled, no storage scope is available: %s", value)
	return false
}

// TokenBlob is used to decode the JSON blob containing an access token
// that is returned by GCE metadata.
type TokenBlob struct {
	AccessToken string `json:"access_token"`
}

// DTO definitions for public GCP STS and IAM token exchanges
type stsTokenExchangeRequest struct {
	Audience           string `json:"audience,omitempty"`
	GrantType          string `json:"grant_type"`
	RequestedTokenType string `json:"requested_token_type"`
	Scope              string `json:"scope,omitempty"`
	SubjectToken       string `json:"subject_token"`
	SubjectTokenType   string `json:"subject_token_type"`
	Options            string `json:"options,omitempty"`
}

type stsTokenExchangeResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

type iamGenerateAccessTokenRequest struct {
	Scope []string `json:"scope"`
}

type iamGenerateAccessTokenResponse struct {
	AccessToken string `json:"accessToken"`
	ExpireTime  string `json:"expireTime"`
}

func logToFile(msg string) {
	logPath := filepath.Join(os.TempDir(), "auth-provider-gcp-wif-poc.log")
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	timestamp := time.Now().Format(time.RFC3339)
	f.WriteString(fmt.Sprintf("[%s] %s\n", timestamp, msg))
}

// Provide implements DockerConfigProvider
func (g *ContainerRegistryProvider) Provide(image string) credentialconfig.DockerConfig {
	cfg := credentialconfig.DockerConfig{}

	// Trigger Workload Identity flow if a ServiceAccountToken is passed by Kubelet
	if g.ServiceAccountToken != "" {
		logToFile(fmt.Sprintf("Triggering auth-provider-gcp POC Workload Identity flow for image: %s", image))
		klog.V(2).Infof("auth-provider-gcp POC: Triggering Workload Identity exchange for image: %s", image)
		accessToken, err := g.executeWorkloadIdentityExchange(context.Background(), image)
		if err == nil {
			logToFile("Workload Identity exchange completely successful, token returned to Kubelet.")
			klog.V(2).Infof("auth-provider-gcp POC: Workload Identity exchange successful!")
			entry := credentialconfig.DockerConfigEntry{
				Username: "_token",
				Password: accessToken,
			}
			g.populateConfig(cfg, image, entry)
			return cfg
		}
		
		// Check if KSA is explicitly opted in to Workload Identity image pulling
		isAnnotated := false
		if g.ServiceAccountAnnotations != nil {
			if _, ok := g.ServiceAccountAnnotations["iam.gke.io/image-pull-service-account"]; ok {
				isAnnotated = true
			}
			if val, ok := g.ServiceAccountAnnotations["iam.gke.io/enable-image-pull-credentials"]; ok && val == "true" {
				isAnnotated = true
			}
		}

		if isAnnotated {
			// Strict Fail-Fast Guardrail: Do NOT fall back to GCE Node SA if exchange fails
			logToFile(fmt.Sprintf("Workload Identity exchange failed for opted-in KSA: %v. Failing fast.", err))
			klog.Errorf("auth-provider-gcp POC: Workload Identity exchange failed for opted-in KSA (%v). Failing fast without fallback.", err)
			return cfg // Returns empty config (causes authentication failure)
		}

		// Fallback Guardrail: If WI exchange fails for unannotated workloads, fallback gracefully to Node SA
		logToFile(fmt.Sprintf("Workload Identity exchange failed: %v. Triggering Node Service Account fallback.", err))
		klog.Warningf("auth-provider-gcp POC: Workload Identity exchange failed (%v). Falling back gracefully to default Node Service Account credentials.", err)
	} else {
		logToFile(fmt.Sprintf("Standard flow active: No ServiceAccountToken received, using Node Service Account for image: %s", image))
	}

	// Standard Fallback: Node-level GCE metadata server token query
	tokenJSONBlob, err := credentialconfig.ReadURL(metadataToken, g.Client, metadataHeader)
	if err != nil {
		klog.Errorf("while reading access token endpoint: %v", err)
		return cfg
	}

	email, err := credentialconfig.ReadURL(metadataEmail, g.Client, metadataHeader)
	if err != nil {
		klog.Errorf("while reading email endpoint: %v", err)
		return cfg
	}

	var parsedBlob TokenBlob
	if err := json.Unmarshal([]byte(tokenJSONBlob), &parsedBlob); err != nil {
		klog.Errorf("while parsing json blob %s: %v", tokenJSONBlob, err)
		return cfg
	}

	entry := credentialconfig.DockerConfigEntry{
		Username: "_token",
		Password: parsedBlob.AccessToken,
		Email:    string(email),
	}

	g.populateConfig(cfg, image, entry)
	return cfg
}

func (g *ContainerRegistryProvider) populateConfig(cfg credentialconfig.DockerConfig, image string, entry credentialconfig.DockerConfigEntry) {
	if g.UseRegistryFromImage {
		if registry, _, found := strings.Cut(image, "/"); found {
			cfg[registry] = entry
		}
	}
	for _, k := range containerRegistryUrls {
		cfg[k] = entry
	}
}

// executeWorkloadIdentityExchange handles the multi-step downscoped exchange pipelines (Direct vs Impersonated)
func (g *ContainerRegistryProvider) executeWorkloadIdentityExchange(ctx context.Context, image string) (string, error) {
	// Extract GSA from annotations to determine mode
	gsaEmail := ""
	if g.ServiceAccountAnnotations != nil {
		gsaEmail = g.ServiceAccountAnnotations["iam.gke.io/gcp-service-account"]
	}

	// Resolve GCP Project ID
	projectID := ""
	if gsaEmail != "" {
		parts := strings.Split(gsaEmail, "@")
		if len(parts) == 2 {
			subparts := strings.Split(parts[1], ".")
			if len(subparts) > 0 {
				projectID = subparts[0]
			}
		}
	}
	
	// Fallback: If GSA is empty (Mode 2: Direct Access), query local Node GCE Metadata Server for true Project ID
	if projectID == "" || projectID == "_" {
		klog.V(4).Infof("auth-provider-gcp POC: GSA annotation absent. Querying GCE Metadata for true Project ID.")
		readID, err := credentialconfig.ReadURL(metadataURL+"project/project-id", g.Client, metadataHeader)
		if err != nil || len(readID) == 0 {
			return "", fmt.Errorf("failed to resolve project-id from GCE metadata: %w", err)
		}
		projectID = string(bytes.TrimSpace(readID))
	}
	
	// Step 1: Trade KSA token for a Google Federated Token via STS
	logToFile(fmt.Sprintf("Executing WIF Step 1: Calling STS API for Project ID: %s", projectID))
	klog.V(2).Infof("auth-provider-gcp POC WIF: Executing Step 1 (STS exchange)")
	federatedToken, err := g.exchangeKSATokenForFederated(ctx, projectID)
	if err != nil {
		return "", fmt.Errorf("failed Step 1 (KSA->Federated STS exchange): %w", err)
	}
	logToFile("WIF Step 1 Successful: Google Federated Token acquired.")

	subjectToken := federatedToken
	// subjectTokenType := "urn:ietf:params:oauth:token-type:access_token"

	// Mode 1: If GSA email annotation is present, execute Step 2 (IAM Impersonation)
	if gsaEmail != "" {
		logToFile(fmt.Sprintf("Executing Mode 1: GSA Impersonation requested targeting: %s", gsaEmail))
		klog.V(2).Infof("auth-provider-gcp POC WIF: Executing Mode 1 (GSA Impersonation) targeting: %s", gsaEmail)
		gsaToken, err := g.impersonateGSA(ctx, federatedToken, gsaEmail)
		if err != nil {
			return "", fmt.Errorf("failed Mode 1 - Step 2 (GSA Impersonation via IAM): %w", err)
		}
		logToFile("Mode 1 - Step 2 Successful: GSA Access Token acquired.")
		subjectToken = gsaToken
	} else {
		logToFile("Executing Mode 2: Direct Access / Principal Federation Mode (No GSA annotation found).")
		klog.V(2).Infof("auth-provider-gcp POC WIF: Executing Mode 2 (Direct Access / Principal Federation)")
	}

	// Step 3: Execute Token Downscoping via Credential Access Boundaries (CAB)
	// Bypassed temporarily for testing to isolate GAR CAB support limits
	logToFile("Bypassing WIF Step 3 (CAB downscoping) temporarily for testing. Returning primary token directly.")
	/*
	logToFile("Executing WIF Step 3: Downscoping Token via Credential Access Boundary (CAB)...")
	klog.V(2).Infof("auth-provider-gcp POC WIF: Executing Step 3 (Credential Access Boundary token downscoping)")
	downscopedToken, err := g.downscopeTokenViaCAB(ctx, subjectToken, subjectTokenType, projectID, image)
	if err != nil {
		return "", fmt.Errorf("failed Step 3 (Token Downscoping via CAB STS exchange): %w", err)
	}
	logToFile("WIF Step 3 Successful: Cryptographically downscoped access token acquired.")
	return downscopedToken, nil
	*/

	return subjectToken, nil
}

func (g *ContainerRegistryProvider) exchangeKSATokenForFederated(ctx context.Context, projectID string) (string, error) {
	// Query the GCE metadata server dynamically to extract this node's cluster traits
	clusterNameBytes, errName := credentialconfig.ReadURL(metadataURL+"instance/attributes/cluster-name", g.Client, metadataHeader)
	if errName != nil || len(clusterNameBytes) == 0 {
		return "", fmt.Errorf("failed to resolve cluster-name from GCE metadata: %w", errName)
	}
	clusterName := string(bytes.TrimSpace(clusterNameBytes))

	clusterLocBytes, errLoc := credentialconfig.ReadURL(metadataURL+"instance/attributes/cluster-location", g.Client, metadataHeader)
	if errLoc != nil || len(clusterLocBytes) == 0 {
		return "", fmt.Errorf("failed to resolve cluster-location from GCE metadata: %w", errLoc)
	}
	clusterLocation := string(bytes.TrimSpace(clusterLocBytes))

	// Format the full-scoped Identity Provider URL required by GCP STS for GKE Workload Identity
	identityProvider := fmt.Sprintf("https://container.googleapis.com/v1/projects/%s/locations/%s/clusters/%s", projectID, clusterLocation, clusterName)
	// Audience format: identitynamespace:<POOL_ID>:<PROVIDER_URL>
	audience := fmt.Sprintf("identitynamespace:%s.svc.id.goog:%s", projectID, identityProvider)
	logToFile(fmt.Sprintf("Constructed STS Full Audience: %s", audience))

	payload := stsTokenExchangeRequest{
		Audience:           audience,
		GrantType:          "urn:ietf:params:oauth:grant-type:token-exchange",
		RequestedTokenType: "urn:ietf:params:oauth:token-type:access_token",
		Scope:              "https://www.googleapis.com/auth/cloud-platform",
		SubjectToken:       g.ServiceAccountToken,
		SubjectTokenType:   "urn:ietf:params:oauth:token-type:jwt",
	}

	return g.callSTSEndpoint(ctx, payload, "")
}

func (g *ContainerRegistryProvider) impersonateGSA(ctx context.Context, federatedToken string, gsaEmail string) (string, error) {
	payload := iamGenerateAccessTokenRequest{
		Scope: []string{"https://www.googleapis.com/auth/cloud-platform"},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/%s:generateAccessToken", gsaEmail)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", federatedToken))

	resp, err := g.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("IAM impersonation failed with status %d: %s", resp.StatusCode, string(respBytes))
	}

	var iamResp iamGenerateAccessTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&iamResp); err != nil {
		return "", err
	}

	return iamResp.AccessToken, nil
}

func (g *ContainerRegistryProvider) downscopeTokenViaCAB(ctx context.Context, subjectToken string, subjectTokenType string, projectID string, image string) (string, error) {
	// Fallback defaults for CAB
	garResource := fmt.Sprintf("//artifactregistry.googleapis.com/projects/%s/locations/*/repositories/*", projectID)
	gcsResource := fmt.Sprintf("//storage.googleapis.com/projects/_/buckets/artifacts.%s.appspot.com", projectID)

	// Parse the image string dynamically to craft tight, explicit resource boundaries
	if imgParts := strings.Split(image, "/"); len(imgParts) >= 3 {
		host := imgParts[0]
		proj := imgParts[1]
		repo := imgParts[2]

		if strings.HasSuffix(host, "-docker.pkg.dev") {
			loc := strings.TrimSuffix(host, "-docker.pkg.dev")
			garResource = fmt.Sprintf("//artifactregistry.googleapis.com/projects/%s/locations/%s/repositories/%s", proj, loc, repo)
			logToFile(fmt.Sprintf("Parsed Explicit GAR Resource Boundary: %s", garResource))
		}
		
		if strings.HasSuffix(host, "gcr.io") {
			gcsResource = fmt.Sprintf("//storage.googleapis.com/projects/_/buckets/%s", host)
		}
	}

	// Craft the raw JSON string for the Credential Access Boundary, combining explicit resource paths with the working role strings
	cabOptions := fmt.Sprintf("{\"accessBoundary\":{\"accessBoundaryRules\":[{\"availableResource\":\"%s\",\"availablePermissions\":[\"inRole:roles/storage.objectViewer\"]},{\"availableResource\":\"%s\",\"availablePermissions\":[\"inRole:roles/artifactregistry.reader\"]}]}}", gcsResource, garResource)

	payload := stsTokenExchangeRequest{
		GrantType:          "urn:ietf:params:oauth:grant-type:token-exchange",
		RequestedTokenType: "urn:ietf:params:oauth:token-type:access_token",
		SubjectToken:       subjectToken,
		SubjectTokenType:   subjectTokenType,
		Options:            cabOptions,
	}

	return g.callSTSEndpoint(ctx, payload, "")
}

func (g *ContainerRegistryProvider) callSTSEndpoint(ctx context.Context, payload stsTokenExchangeRequest, bearerToken string) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://sts.googleapis.com/v1/token", bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if bearerToken != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", bearerToken))
	}

	resp, err := g.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("STS token exchange failed with status %d: %s", resp.StatusCode, string(respBytes))
	}

	var stsResp stsTokenExchangeResponse
	if err := json.NewDecoder(resp.Body).Decode(&stsResp); err != nil {
		return "", err
	}

	return stsResp.AccessToken, nil
}

