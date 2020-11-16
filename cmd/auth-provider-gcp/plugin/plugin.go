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
	"fmt"
	utilnet "k8s.io/apimachinery/pkg/util/net"
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

func GetResponse(image string, metadataURL string, storageScopePrefix string, cloudScope string) (*Response, error) {
	fmt.Printf("metadataURL: %s\n", metadataURL)
	fmt.Printf("storageScopePrefix: %s\n", storageScopePrefix)
	fmt.Printf("cloudPlatformScope: %s\n", cloudScope)
	tr := utilnet.SetTransportDefaults(&http.Transport{})
	metadataHTTPClientTimeout := time.Second * 10
	httpClient := &http.Client{
		Transport: tr,
		Timeout:   metadataHTTPClientTimeout,
	}
	provider := &gcpcredential.ContainerRegistryProvider{
		gcpcredential.MetadataProvider{Client: httpClient},
	}
	cfg := provider.Provide(image)
	username := cfg["gcr.io"].Username
	password := cfg["gcr.io"].Password
	return &Response{Username: username, Password: password}, nil
}
