/*
Copyright 2020 The Kubernetes Authors.

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

package plugin

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cloud-provider-gcp/cmd/auth-provider-gcp/credentialconfig"
	"k8s.io/cloud-provider-gcp/cmd/auth-provider-gcp/gcpcredential"
	credentialproviderapi "k8s.io/kubelet/pkg/apis/credentialprovider"
	"net/http"
	"os"
	"time"
)

const (
	cacheDurationKey          = "KUBE_SIDECAR_CACHE_DURATION"
	metadataHTTPClientTimeout = time.Second * 10
)

// MakeRegistryProvider returns a ContainerRegistryProvider with the given transport.
func MakeRegistryProvider(transport *http.Transport) *gcpcredential.ContainerRegistryProvider {
	httpClient := &http.Client{
		Transport: transport,
		Timeout:   metadataHTTPClientTimeout,
	}
	provider := &gcpcredential.ContainerRegistryProvider{
		gcpcredential.MetadataProvider{Client: httpClient},
	}
	return provider
}

// MakeDockerConfigProvider returns a DockerConfigKeyProvider with the given transport.
func MakeDockerConfigProvider(transport *http.Transport) *gcpcredential.DockerConfigKeyProvider {
	httpClient := &http.Client{
		Transport: transport,
		Timeout:   metadataHTTPClientTimeout,
	}
	provider := &gcpcredential.DockerConfigKeyProvider{
		gcpcredential.MetadataProvider{Client: httpClient},
	}
	return provider
}

// MakeDockerConfigURLProvider returns a DockerConfigURLKeyProvider with the given transport.
func MakeDockerConfigURLProvider(transport *http.Transport) *gcpcredential.DockerConfigURLKeyProvider {
	httpClient := &http.Client{
		Transport: transport,
		Timeout:   metadataHTTPClientTimeout,
	}
	provider := &gcpcredential.DockerConfigURLKeyProvider{
		gcpcredential.MetadataProvider{Client: httpClient},
	}
	return provider
}

// GetResponse queries the given provider for credentials.
func GetResponse(provider credentialconfig.DockerConfigProvider) (*credentialproviderapi.CredentialProviderResponse, error) {
	// pass an empty image string to Provide() - the image name is not actually used
	cfg := provider.Provide("")
	response := &credentialproviderapi.CredentialProviderResponse{Auth: make(map[string]credentialproviderapi.AuthConfig)}
	for url, dockerConfig := range cfg {
		response.Auth[url] = credentialproviderapi.AuthConfig{Username: dockerConfig.Username, Password: dockerConfig.Password}
	}
	unparsedCacheDuration := os.Getenv(cacheDurationKey)
	if unparsedCacheDuration != "" {
		cacheDuration, err := time.ParseDuration(unparsedCacheDuration)
		if err != nil {
			return nil, err
		}
		response.CacheDuration = &metav1.Duration{cacheDuration}
	}
	return response, nil
}