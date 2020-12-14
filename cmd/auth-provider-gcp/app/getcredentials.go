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

package app

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"io/ioutil"
	"net/http"
	"os"

	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/cloud-provider-gcp/cmd/auth-provider-gcp/provider"
	credentialproviderapi "k8s.io/cloud-provider-gcp/pkg/apis/credentialprovider"
	"k8s.io/cloud-provider-gcp/pkg/credentialconfig"
	klog "k8s.io/klog/v2"
)

var (
	authFlow string
)

const (
	gcrAuthFlow             = "gcr"
	dockerConfigAuthFlow    = "dockercfg"
	dockerConfigURLAuthFlow = "dockercfg-url"
)

// NewGetCredentialsCommand returns a cobra command that retrieves auth credentials after validating flags.
func NewGetCredentialsCommand() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "get-credentials",
		Short: "Get authentication credentials",
		RunE:  getCredentials,
	}
	defineFlags(cmd)
	if err := validateFlags(authFlow); err != nil {
		return nil, err
	}
	return cmd, nil
}

func providerFromFlow(flow string) (credentialconfig.DockerConfigProvider, error) {
	transport := utilnet.SetTransportDefaults(&http.Transport{})
	switch flow {
	case gcrAuthFlow:
		return provider.MakeRegistryProvider(transport), nil
	case dockerConfigAuthFlow:
		return provider.MakeDockerConfigProvider(transport), nil
	case dockerConfigURLAuthFlow:
		return provider.MakeDockerConfigURLProvider(transport), nil
	default:
		return nil, fmt.Errorf("unrecognized auth flow \"%s\"", flow)
	}
}

func getCredentials(cmd *cobra.Command, args []string) error {
	klog.V(2).Infof("get-credentials %s", authFlow)
	authProvider, err := providerFromFlow(authFlow)
	if err != nil {
		return err
	}
	unparsedRequest, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		return err
	}
	var authRequest credentialproviderapi.CredentialProviderRequest
	err = json.Unmarshal(unparsedRequest, &authRequest)
	if err != nil {
		return fmt.Errorf("error unmarshaling auth credential request: %w", err)
	}
	authCredentials, err := provider.GetResponse(authRequest.Image, authProvider)
	if err != nil {
		return fmt.Errorf("error getting authentication response from provider: %w", err)
	}
	jsonResponse, err := json.Marshal(authCredentials)
	if err != nil {
		// The error from json.Marshal is intentionally not included so as to not leak credentials into the logs
		return fmt.Errorf("error marshaling credentials")
	}
	// Emit authentication response for kubelet to consume
	fmt.Println(string(jsonResponse))
	return nil
}

func defineFlags(credCmd *cobra.Command) {
	credCmd.Flags().StringVarP(&authFlow, "authFlow", "a", gcrAuthFlow, "authentication flow (valid values are gcr, dockercfg, and dockercfg-url)")
}

func validateFlags(flow string) error {
	if flow != gcrAuthFlow && flow != dockerConfigAuthFlow && flow != dockerConfigURLAuthFlow {
		return fmt.Errorf("invalid value %q for authFlow (must be one of %q, %q, or %q)", flow, gcrAuthFlow, dockerConfigAuthFlow, dockerConfigURLAuthFlow)
	}
	return nil
}
