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

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

// Command-line argument names that can be used to configure base URLs. The
// keys correspond to the constants in credentialprovider/gcp/metadata.go.
const (
	metadataUrlArg              = "metadataURL"
	storageScopePrefixArg       = "storageScopePrefix"
	cloudPlatformScopePrefixArg = "cloudPlatformScope"
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
type gcrPlugin struct {
}

func (g *gcrPlugin) GetAuthPluginResponse(image string, extraArgs map[string]string) (*pluginResponse, error) {
	metadataURL, ok := extraArgs[metadataUrlArg]
	if !ok {
		return nil, fmt.Errorf("metadata URL not provided (%s option)", metadataUrlArg)
	}
	storageScopePrefix, ok := extraArgs[storageScopePrefixArg]
	if !ok {
		return nil, fmt.Errorf("storage scope prefix not provided (%s option)", storageScopePrefixArg)
	}
	cloudScope, ok := extraArgs[cloudPlatformScopePrefixArg]
	if !ok {
		return nil, fmt.Errorf("cloud platfom scope not provided (%s option)", cloudPlatformScopePrefixArg)
	}
	fmt.Printf("metadataURL: %s\n", metadataURL)
	fmt.Printf("storageScopePrefix: %s\n", storageScopePrefix)
	fmt.Printf("cloudPlatformScope: %s\n", cloudScope)
	return &pluginResponse{Username: "testuser", Password: "testpass"}, nil
}

// taken from:
// https://github.com/andrewsykim/kubernetes/blob/33eff26e6e2a3271c9fcd6b727eef63586700dec/pkg/credentialprovider/plugin/framework/plugin.go#L63
// TODO(DangerOnTheRanger): remove this when importing credentialprovider
func parseArgs(args []string) (map[string]string, error) {
	if len(args) == 0 {
		return nil, errors.New("no arguments were provided to plugin")
	}

	if args[0] != "get-credentials" {
		return nil, fmt.Errorf("invalid plugin argument %s, only 'get-credentials' is supported", args[0])
	}

	extraArgs := make(map[string]string)
	for _, extraArg := range args[1:] {
		argSplit := strings.Split(extraArg, "=")

		// we always expect extra-args to be in the format "key=value"
		if len(argSplit) != 2 {
			return nil, fmt.Errorf("unexpected argument format: %s", extraArg)
		}

		key := argSplit[0]
		value := argSplit[1]
		extraArgs[key] = value
	}

	return extraArgs, nil
}

// Simplified reimplementation of CRIAuthPlugin.Run. This function handles
// command-line argument parsing and some validation normally handled by the
// other function.
func runPlugin(rawArgs []string) int {
	pluginArgs, err := parseArgs(rawArgs)
	if err != nil {
		fmt.Printf("error while parsing args: %s\n", err.Error())
		return 1
	}
	plugin := &gcrPlugin{}
	// TODO(DangerOnTheRanger): don't use hardcoded image name
	image := "hello-world"
	authCredentials, err := plugin.GetAuthPluginResponse(image, pluginArgs)
	if err != nil {
		fmt.Printf("error while obtaining credentials: %s\n", err.Error())
		return 1
	}
	jsonResponse, err := json.Marshal(authCredentials)
	if err != nil {
		fmt.Printf("error while marshaling json: %s\n", err.Error())
		return 1
	}
	fmt.Println(string(jsonResponse))
	return 0
}

func main() {
	os.Exit(runPlugin(os.Args[1:]))
}
