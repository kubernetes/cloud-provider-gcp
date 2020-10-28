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
)

// TODO(DangerOnTheRanger): temporary structure until credentialprovider
// is built with cloud-provider-gcp; GetAuthPluginResponse should return
// CRIAuthPluginResponse instead, but this should be nearly a drop-in replacement
type pluginResponse struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Empty type with a single function GetAuthPluginResponse. Required by the
// CRI auth plugin framework.
type GCRPlugin struct {
}

func (g *GCRPlugin) GetAuthPluginResponse(image string, metadataURL string, storageScopePrefix string, cloudScope string) (*pluginResponse, error) {
	fmt.Printf("metadataURL: %s\n", metadataURL)
	fmt.Printf("storageScopePrefix: %s\n", storageScopePrefix)
	fmt.Printf("cloudPlatformScope: %s\n", cloudScope)
	return &pluginResponse{Username: "testuser", Password: "testpass"}, nil
}
