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
	"io/ioutil"
	"net/http"
	"os"

	"github.com/spf13/cobra"

	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/cloud-provider-gcp/cmd/auth-provider-gcp/provider"
	credentialproviderapi "k8s.io/cloud-provider-gcp/pkg/apis/credentialprovider"
	"k8s.io/cloud-provider-gcp/pkg/credentialconfig"
	klog "k8s.io/klog/v2"
)

const (
	gcrAuthFlow             = "gcr"
	dockerConfigAuthFlow    = "dockercfg"
	dockerConfigURLAuthFlow = "dockercfg-url"
)

type CredentialOptions struct {
	AuthFlow string
}

type AuthFlowFlagError struct {
	flagValue string
}

func (a *AuthFlowFlagError) Error() string {
	return fmt.Sprintf("invalid value %q for authFlow (must be one of %q, %q, or %q)", a.flagValue, gcrAuthFlow, dockerConfigAuthFlow, dockerConfigURLAuthFlow)
}

func (a *AuthFlowFlagError) Is(err error) bool {
	_, ok := err.(*AuthFlowFlagError)
	return ok
}

type AuthFlowTypeError struct {
	requestedFlow string
}

func (p *AuthFlowTypeError) Error() string {
	return fmt.Sprintf("unrecognized auth flow %q", p.requestedFlow)
}

func (p *AuthFlowTypeError) Is(err error) bool {
	_, ok := err.(*AuthFlowTypeError)
	return ok
}

// NewGetCredentialsCommand returns a cobra command that retrieves auth credentials after validating flags.
func NewGetCredentialsCommand() (*cobra.Command, error) {
	var options CredentialOptions
	cmd := &cobra.Command{
		Use:   "get-credentials",
		Short: "Get authentication credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			return getCredentials(options.AuthFlow)
		},
	}
	defineFlags(cmd, &options)
	if err := validateFlags(&options); err != nil {
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
		return nil, &AuthFlowTypeError{requestedFlow: flow}
	}
}

func getCredentials(authFlow string) error {
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

func defineFlags(credCmd *cobra.Command, options *CredentialOptions) {
	credCmd.Flags().StringVarP(&options.AuthFlow, "authFlow", "a", gcrAuthFlow, fmt.Sprintf("authentication flow (valid values are %q, %q, and %q)", gcrAuthFlow, dockerConfigAuthFlow, dockerConfigURLAuthFlow))
}

func validateFlags(options *CredentialOptions) error {
	if options.AuthFlow != gcrAuthFlow && options.AuthFlow != dockerConfigAuthFlow && options.AuthFlow != dockerConfigURLAuthFlow {
		return &AuthFlowFlagError{flagValue: options.AuthFlow}
	}
	return nil
}
