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

package provider

import (
	"net/http"
	"os"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	credentialproviderapi "k8s.io/cloud-provider-gcp/pkg/apis/credentialprovider"
	"k8s.io/cloud-provider-gcp/pkg/credentialconfig"
	"k8s.io/cloud-provider-gcp/pkg/gcpcredential"
)

const (
	cacheDurationKey          = "KUBE_SIDECAR_CACHE_DURATION"
	metadataHTTPClientTimeout = time.Second * 10
	apiKind                   = "CredentialProviderResponse"
	apiVersion                = "credentialprovider.kubelet.k8s.io/v1alpha1"
)

// MakeRegistryProvider returns a ContainerRegistryProvider with the given transport.
func MakeRegistryProvider(transport *http.Transport) *gcpcredential.ContainerRegistryProvider {
	httpClient := makeHTTPClient(transport)
	provider := &gcpcredential.ContainerRegistryProvider{
		gcpcredential.MetadataProvider{Client: httpClient},
	}
	return provider
}

// MakeDockerConfigProvider returns a DockerConfigKeyProvider with the given transport.
func MakeDockerConfigProvider(transport *http.Transport) *gcpcredential.DockerConfigKeyProvider {
	httpClient := makeHTTPClient(transport)
	provider := &gcpcredential.DockerConfigKeyProvider{
		gcpcredential.MetadataProvider{Client: httpClient},
	}
	return provider
}

// MakeDockerConfigURLProvider returns a DockerConfigURLKeyProvider with the given transport.
func MakeDockerConfigURLProvider(transport *http.Transport) *gcpcredential.DockerConfigURLKeyProvider {
	httpClient := makeHTTPClient(transport)
	provider := &gcpcredential.DockerConfigURLKeyProvider{
		gcpcredential.MetadataProvider{Client: httpClient},
	}
	return provider
}

func makeHTTPClient(transport *http.Transport) *http.Client {
	return &http.Client{
		Transport: transport,
		Timeout:   metadataHTTPClientTimeout,
	}
}

func getCacheDuration() (time.Duration, error) {
	unparsedCacheDuration := os.Getenv(cacheDurationKey)
	if unparsedCacheDuration == "" {
		return 0, nil
	} else {
		cacheDuration, err := time.ParseDuration(unparsedCacheDuration)
		if err != nil {
			return 0, err
		}
		return cacheDuration, nil
	}
}

// GetResponse queries the given provider for credentials.
func GetResponse(image string, provider credentialconfig.DockerConfigProvider) (*credentialproviderapi.CredentialProviderResponse, error) {
	cfg := provider.Provide(image)
	response := &credentialproviderapi.CredentialProviderResponse{Auth: make(map[string]credentialproviderapi.AuthConfig)}
	for url, dockerConfig := range cfg {
		response.Auth[url] = credentialproviderapi.AuthConfig{Username: dockerConfig.Username, Password: dockerConfig.Password}
	}
	cacheDuration, err := getCacheDuration()
	if err != nil {
		return nil, err
	}
	if cacheDuration != 0 {
		response.CacheDuration = &metav1.Duration{cacheDuration}
	}
	response.TypeMeta.Kind = apiKind
	response.TypeMeta.APIVersion = apiVersion
	return response, nil
}
