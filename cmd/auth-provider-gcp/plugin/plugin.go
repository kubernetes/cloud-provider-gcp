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
	utilnet "k8s.io/apimachinery/pkg/util/net"
	credentialproviderapi "k8s.io/kubelet/pkg/apis/credentialprovider"
	"k8s.io/cloud-provider-gcp/cmd/auth-provider-gcp/gcpcredential"
	"net/http"
	"time"
)

// TODO(DangerOnTheRanger): temporary structure until credentialprovider
// is built with cloud-provider-gcp; GetAuthPluginResponse should return
// CRIAuthPluginResponse instead, but this should be nearly a drop-in replacement
type Response struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func GetResponse(metadataURL string, storageScopePrefix string, cloudScope string) (*credentialproviderapi.CredentialProviderResponse, error) {
	tr := utilnet.SetTransportDefaults(&http.Transport{})
	metadataHTTPClientTimeout := time.Second * 10
	httpClient := &http.Client{
		Transport: tr,
		Timeout:   metadataHTTPClientTimeout,
	}
	provider := &gcpcredential.ContainerRegistryProvider{
		gcpcredential.MetadataProvider{Client: httpClient},
	}
	// pass an image string to Provide() - the image name is not actually used
	cfg := provider.Provide("")
	response := &credentialproviderapi.CredentialProviderResponse{Auth: make(map[string]credentialproviderapi.AuthConfig)}
	for url, dockerConfig := range cfg {
		response.Auth[url] = credentialproviderapi.AuthConfig{Username: dockerConfig.Username, Password: dockerConfig.Password}
	}
	return response, nil
}
