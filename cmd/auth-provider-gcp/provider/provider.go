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
	"fmt"
	"net/http"
	"os"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cloud-provider-gcp/pkg/credentialconfig"
	"k8s.io/cloud-provider-gcp/pkg/gcpcredential"
	"k8s.io/kubelet/pkg/apis/credentialprovider"
	credentialproviderapi "k8s.io/kubelet/pkg/apis/credentialprovider"
)

const (
	cacheImage                = "image"
	cacheRegistry             = "registry"
	cacheGlobal               = "global"
	cacheDurationKey          = "KUBE_SIDECAR_CACHE_DURATION"
	cacheTypeKey              = "KUBE_SIDECAR_CACHE_TYPE"
	metadataHTTPClientTimeout = time.Second * 10
	apiKind                   = "CredentialProviderResponse"
	apiVersion                = "credentialprovider.kubelet.k8s.io/v1alpha1"
)

// MakeRegistryProvider returns a ContainerRegistryProvider with the given transport.
func MakeRegistryProvider(transport *http.Transport) *gcpcredential.ContainerRegistryProvider {
	httpClient := makeHTTPClient(transport)
	provider := &gcpcredential.ContainerRegistryProvider{
		MetadataProvider: gcpcredential.MetadataProvider{Client: httpClient},
	}
	return provider
}

// MakeDockerConfigProvider returns a DockerConfigKeyProvider with the given transport.
func MakeDockerConfigProvider(transport *http.Transport) *gcpcredential.DockerConfigKeyProvider {
	httpClient := makeHTTPClient(transport)
	provider := &gcpcredential.DockerConfigKeyProvider{
		MetadataProvider: gcpcredential.MetadataProvider{Client: httpClient},
	}
	return provider
}

// MakeDockerConfigURLProvider returns a DockerConfigURLKeyProvider with the given transport.
func MakeDockerConfigURLProvider(transport *http.Transport) *gcpcredential.DockerConfigURLKeyProvider {
	httpClient := makeHTTPClient(transport)
	provider := &gcpcredential.DockerConfigURLKeyProvider{
		MetadataProvider: gcpcredential.MetadataProvider{Client: httpClient},
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
		// a value of 0 for the cache duration will result in the credentials not being cached
		// at all, which is equivalent to what the in-tree provider does; since the
		// KUBE_SIDECAR_CACHE_DURATION environment variable is not set by default,
		// backwards compatibility is maintained by default
		return 0, nil
	}
	cacheDuration, err := time.ParseDuration(unparsedCacheDuration)
	if err != nil {
		return 0, err
	}
	return cacheDuration, nil
}

func getCacheKeyType() (credentialprovider.PluginCacheKeyType, error) {
	keyType := os.Getenv(cacheTypeKey)
	if keyType == "" {
		return credentialproviderapi.ImagePluginCacheKeyType, nil
	}
	switch keyType {
	case cacheImage:
		return credentialprovider.ImagePluginCacheKeyType, nil
	case cacheRegistry:
		return credentialprovider.RegistryPluginCacheKeyType, nil
	case cacheGlobal:
		return credentialprovider.GlobalPluginCacheKeyType, nil
	default:
		var nilKeyType credentialprovider.PluginCacheKeyType = ""
		return nilKeyType, fmt.Errorf("Unknown cache key %q", keyType)
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
	response.CacheDuration = &metav1.Duration{Duration: cacheDuration}
	response.TypeMeta.Kind = apiKind
	response.TypeMeta.APIVersion = apiVersion
	cacheKey, err := getCacheKeyType()
	if err != nil {
		return nil, err
	}
	response.CacheKeyType = cacheKey
	return response, nil
}
