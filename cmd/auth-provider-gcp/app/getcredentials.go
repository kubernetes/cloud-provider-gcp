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
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/spf13/cobra"

	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/cloud-provider-gcp/cmd/auth-provider-gcp/provider"
	"k8s.io/cloud-provider-gcp/pkg/credentialconfig"
	"k8s.io/cloud-provider-gcp/pkg/gcpcredential"
	klog "k8s.io/klog/v2"
	credentialproviderapi "k8s.io/kubelet/pkg/apis/credentialprovider/v1"
)

const (
	gcrAuthFlow             = "gcr"
	dockerConfigAuthFlow    = "dockercfg"
	dockerConfigURLAuthFlow = "dockercfg-url"
)

// CredentialOptions contains a representation of the options passed to the credential provider.
type CredentialOptions struct {
	AuthFlow string
	K8sType  string
	gcpcredential.WIConfig
}

// ErrProjectIDRequired is returned when GKE Workload Identity uses --identity-provider without --project-id.
var ErrProjectIDRequired = errors.New("--project-id is required when --identity-provider is set")

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

// K8sTypeFlagError represents an error that occurred during k8sType flag validation.
type K8sTypeFlagError struct {
	flagValue string
}

// Error implements error.Error.
func (k *K8sTypeFlagError) Error() string {
	return fmt.Sprintf("invalid value %q for k8sType (must be one of %q or %q)", k.flagValue, gcpcredential.K8sTypeGKE, gcpcredential.K8sTypeSelfManaged)
}

// Is implements the Is function that errors.Is checks for.
func (k *K8sTypeFlagError) Is(err error) bool {
	_, ok := err.(*K8sTypeFlagError)
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
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return validateFlags(&options)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return getCredentials(options)
		},
	}
	defineFlags(cmd, &options)
	return cmd, nil
}

func makeProvider(req credentialproviderapi.CredentialProviderRequest, options CredentialOptions) (credentialconfig.DockerConfigProvider, error) {
	transport := utilnet.SetTransportDefaults(&http.Transport{})
	switch options.AuthFlow {
	case gcrAuthFlow:
		return provider.MakeRegistryProvider(transport, req, options.K8sType, options.WIConfig), nil
	case dockerConfigAuthFlow:
		return provider.MakeDockerConfigProvider(transport), nil
	case dockerConfigURLAuthFlow:
		return provider.MakeDockerConfigURLProvider(transport), nil
	default:
		return nil, &AuthFlowTypeError{requestedFlow: options.AuthFlow}
	}
}

func getCredentials(options CredentialOptions) error {
	klog.V(2).Infof("get-credentials (authFlow %s)", options.AuthFlow)
	if options.AuthFlow == gcrAuthFlow && options.K8sType == gcpcredential.K8sTypeSelfManaged {
		klog.V(2).Infof("auth-provider-gcp: Workload Identity flow is enabled (k8sType: %q)", options.K8sType)
	} else if options.AuthFlow == gcrAuthFlow && options.K8sType == gcpcredential.K8sTypeGKE && options.IdentityProvider != "" {
		klog.V(2).Infof("auth-provider-gcp: Workload Identity flow is enabled (identity-provider: %q)", options.IdentityProvider)
	}
	unparsedRequest, err := io.ReadAll(os.Stdin)
	if err != nil {
		return err
	}
	var authRequest credentialproviderapi.CredentialProviderRequest
	err = json.Unmarshal(unparsedRequest, &authRequest)
	if err != nil {
		return fmt.Errorf("error unmarshaling auth credential request: %w", err)
	}
	authProvider, err := makeProvider(authRequest, options)
	if err != nil {
		return err
	}
	authCredentials, err := provider.GetResponse(authRequest, authProvider)
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
	credCmd.Flags().StringVar(&options.K8sType, "k8sType", gcpcredential.K8sTypeGKE, fmt.Sprintf("Kubernetes cluster type (valid values are %q and %q)", gcpcredential.K8sTypeGKE, gcpcredential.K8sTypeSelfManaged))
	credCmd.Flags().StringVar(&options.IdentityProvider, "identity-provider", "", fmt.Sprintf("Target Identity Provider URL to request federated tokens from. Only takes effect when --authFlow=%q and --k8sType=%q. If configured, it enables Workload Identity flow.", gcrAuthFlow, gcpcredential.K8sTypeGKE))
	credCmd.Flags().StringVar(&options.ProjectID, "project-id", "", fmt.Sprintf("The GCP Project ID. Required when --authFlow=%q, --k8sType=%q, and --identity-provider is configured.", gcrAuthFlow, gcpcredential.K8sTypeGKE))
	credCmd.Flags().StringVar(&options.ProjectNumber, "project-number", "", fmt.Sprintf("The GCP Project Number. Required when --authFlow=%q and --k8sType=%q", gcrAuthFlow, gcpcredential.K8sTypeSelfManaged))
	credCmd.Flags().StringVar(&options.PoolID, "pool-id", "", fmt.Sprintf("The GCP Workload Identity Pool ID. Required when --authFlow=%q and --k8sType=%q", gcrAuthFlow, gcpcredential.K8sTypeSelfManaged))
	credCmd.Flags().StringVar(&options.ProviderID, "provider-id", "", fmt.Sprintf("The GCP Workload Identity Provider ID. Required when --authFlow=%q and --k8sType=%q", gcrAuthFlow, gcpcredential.K8sTypeSelfManaged))
}

func validateFlags(options *CredentialOptions) error {
	if options.AuthFlow != gcrAuthFlow && options.AuthFlow != dockerConfigAuthFlow && options.AuthFlow != dockerConfigURLAuthFlow {
		return &AuthFlowFlagError{flagValue: options.AuthFlow}
	}
	if !gcpcredential.IsValidK8sType(options.K8sType) {
		return &K8sTypeFlagError{flagValue: options.K8sType}
	}
	if options.IdentityProvider != "" && (options.AuthFlow != gcrAuthFlow || options.K8sType != gcpcredential.K8sTypeGKE) {
		klog.Warningf("auth-provider-gcp: --identity-provider was set but --authFlow is %q and --k8sType is %q. This flag only has effect when --authFlow=%q and --k8sType=%q.", options.AuthFlow, options.K8sType, gcrAuthFlow, gcpcredential.K8sTypeGKE)
	}
	if options.K8sType == gcpcredential.K8sTypeSelfManaged && options.AuthFlow != gcrAuthFlow {
		klog.Warningf("auth-provider-gcp: --k8sType is %q but --authFlow is %q. This flag only has effect when --authFlow=%q.", options.K8sType, options.AuthFlow, gcrAuthFlow)
	}
	if options.AuthFlow == gcrAuthFlow && options.K8sType == gcpcredential.K8sTypeGKE && options.IdentityProvider != "" && options.ProjectID == "" {
		return ErrProjectIDRequired
	}
	if options.AuthFlow == gcrAuthFlow && options.K8sType == gcpcredential.K8sTypeSelfManaged && (options.ProjectNumber == "" || options.PoolID == "" || options.ProviderID == "") {
		return fmt.Errorf("when --k8sType=%q, --project-number, --pool-id, and --provider-id must all be set", gcpcredential.K8sTypeSelfManaged)
	}
	return nil
}
