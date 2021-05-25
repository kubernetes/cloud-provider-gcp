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
	stdjson "encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/cloud-provider-gcp/cmd/auth-provider-gcp/provider"
	"k8s.io/cloud-provider-gcp/pkg/credentialconfig"
	klog "k8s.io/klog/v2"
	credentialproviderapi "k8s.io/kubelet/pkg/apis/credentialprovider"
)

const (
	gcrAuthFlow             = "gcr"
	dockerConfigAuthFlow    = "dockercfg"
	dockerConfigURLAuthFlow = "dockercfg-url"
)

// CredentialOptions contains a representation of the options passed to the credential provider.
type CredentialOptions struct {
	AuthFlow string
}

// AuthFlowFlagError represents an error that occurred during flag validation.
type AuthFlowFlagError struct {
	flagValue string
}

// Error implements error.Error.
func (a *AuthFlowFlagError) Error() string {
	return fmt.Sprintf("invalid value %q for authFlow (must be one of %q, %q, or %q)", a.flagValue, gcrAuthFlow, dockerConfigAuthFlow, dockerConfigURLAuthFlow)
}

// Is implements the Is function that errors.Is checks for.
func (a *AuthFlowFlagError) Is(err error) bool {
	_, ok := err.(*AuthFlowFlagError)
	return ok
}

// AuthFlowTypeError represents an indication that an unrecognized auth flow was passed.
type AuthFlowTypeError struct {
	requestedFlow string
}

// Error implements error.Error.
func (p *AuthFlowTypeError) Error() string {
	return fmt.Sprintf("unrecognized auth flow %q", p.requestedFlow)
}

// Is implements the Is function that errors.Is checks for.
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
	klog.V(2).Infof("get-credentials (authFlow %s)", authFlow)
	authProvider, err := providerFromFlow(authFlow)
	if err != nil {
		return err
	}
	unparsedRequest, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		return err
	}
	var authRequest credentialproviderapi.CredentialProviderRequest
	err = stdjson.Unmarshal(unparsedRequest, &authRequest)
	if err != nil {
		return fmt.Errorf("error unmarshaling auth credential request: %w", err)
	}
	authCredentials, err := provider.GetResponse(authRequest.Image, authProvider)
	if err != nil {
		return fmt.Errorf("error getting authentication response from provider: %w", err)
	}
	jsonResponse, err := encodeCredentials(authCredentials)
	if err != nil {
		return fmt.Errorf(err.Error())
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

func encodeCredentials(credentials *credentialproviderapi.CredentialProviderResponse) ([]byte, error) {
	mediaType := "application/json"
	codecs := serializer.NewCodecFactory(runtime.NewScheme())
	info, ok := runtime.SerializerInfoForMediaType(codecs.SupportedMediaTypes(), mediaType)
	if !ok {
		return nil, fmt.Errorf("unsupported media type %q", mediaType)
	}

	encoder := codecs.EncoderForVersion(info.Serializer, credentialproviderapi.SchemeGroupVersion)
	data, err := runtime.Encode(encoder, credentials)
	if err != nil {
		// Intentionally do not include encoding errors to avoid leaking credentials into the logs
		return nil, fmt.Errorf("failed to encode response: %v", err)
	}
	return data, nil
}
