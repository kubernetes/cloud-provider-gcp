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

package gcpcredential

import (
	"context"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"k8s.io/cloud-provider-gcp/pkg/credentialconfig"
	"k8s.io/klog/v2"
)

const cloudPlatformScope = "https://www.googleapis.com/auth/cloud-platform"

// ApplicationDefaultCredentialsProvider is a DockerConfigProvider that provides a dockercfg with:
//
//	Username: "_token"
//	Password: "{access token from Application Default Credentials}"
//
// Credentials are discovered using the standard Application Default Credentials (ADC)
// search order: the file named by the GOOGLE_APPLICATION_CREDENTIALS environment variable,
// the gcloud well-known credentials file, and finally the GCE metadata server.
// This allows any credential type supported by ADC to be used, such as service account keys,
// service account impersonation, and Workload Identity Federation (external account) credentials.
// See https://cloud.google.com/docs/authentication/application-default-credentials
type ApplicationDefaultCredentialsProvider struct {
	// Client is the HTTP client used to fetch tokens. If nil, http.DefaultClient is used.
	Client               *http.Client
	UseRegistryFromImage bool
}

// Enabled implements DockerConfigProvider. Credentials are discovered when Provide is called.
func (g *ApplicationDefaultCredentialsProvider) Enabled() bool {
	return true
}

// Provide implements DockerConfigProvider
func (g *ApplicationDefaultCredentialsProvider) Provide(image string) credentialconfig.DockerConfig {
	cfg := credentialconfig.DockerConfig{}

	ctx := context.Background()
	if g.Client != nil {
		ctx = context.WithValue(ctx, oauth2.HTTPClient, g.Client)
	}
	creds, err := google.FindDefaultCredentials(ctx, cloudPlatformScope)
	if err != nil {
		klog.Errorf("while finding application default credentials: %v", err)
		return cfg
	}
	token, err := creds.TokenSource.Token()
	if err != nil {
		klog.Errorf("while fetching access token from application default credentials: %v", err)
		return cfg
	}

	entry := credentialconfig.DockerConfigEntry{
		Username: "_token",
		Password: token.AccessToken,
	}
	populateRegistryConfig(cfg, image, entry, g.UseRegistryFromImage)
	return cfg
}
