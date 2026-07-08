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
	"errors"
	"reflect"
	"strings"
	"testing"

	credentialproviderapi "k8s.io/kubelet/pkg/apis/credentialprovider/v1"
)

func TestValidateAuthFlow(t *testing.T) {
	type FlagResult struct {
		Name    string
		Options CredentialOptions
		Error   error
	}
	tests := []FlagResult{
		{Name: "validate gcr auth flow", Options: CredentialOptions{AuthFlow: gcrAuthFlow}},
		{Name: "validate docker-cfg auth flow option", Options: CredentialOptions{AuthFlow: dockerConfigAuthFlow}},
		{Name: "validate docker-cfg-url auth flow option", Options: CredentialOptions{AuthFlow: dockerConfigURLAuthFlow}},
		{Name: "bad auth flow option", Options: CredentialOptions{AuthFlow: "bad-flow"}, Error: &AuthFlowFlagError{flagValue: "bad-flow"}},
		{Name: "empty auth flow option", Options: CredentialOptions{AuthFlow: ""}, Error: &AuthFlowFlagError{flagValue: ""}},
		{Name: "case-sensitive auth flow", Options: CredentialOptions{AuthFlow: "Gcrauthflow"}, Error: &AuthFlowFlagError{flagValue: "Gcrauthflow"}},
		{Name: "identity-provider and project-id with gcr flow", Options: CredentialOptions{AuthFlow: gcrAuthFlow, IdentityProvider: "https://container.googleapis.com/...", ProjectID: "my-project"}},
		{Name: "identity-provider and project-id with dockercfg flow", Options: CredentialOptions{AuthFlow: dockerConfigAuthFlow, IdentityProvider: "https://container.googleapis.com/...", ProjectID: "my-project"}},
		{Name: "identity-provider and project-id with dockercfg-url flow", Options: CredentialOptions{AuthFlow: dockerConfigURLAuthFlow, IdentityProvider: "https://container.googleapis.com/...", ProjectID: "my-project"}},
		{Name: "identity-provider without project-id with gcr flow", Options: CredentialOptions{AuthFlow: gcrAuthFlow, IdentityProvider: "https://container.googleapis.com/..."}, Error: ErrProjectIDRequired},
		{Name: "identity-provider without project-id with dockercfg flow", Options: CredentialOptions{AuthFlow: dockerConfigAuthFlow, IdentityProvider: "https://container.googleapis.com/..."}, Error: ErrProjectIDRequired},
		{Name: "identity-provider without project-id with dockercfg-url flow", Options: CredentialOptions{AuthFlow: dockerConfigURLAuthFlow, IdentityProvider: "https://container.googleapis.com/..."}, Error: ErrProjectIDRequired},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			err := validateFlags(&tc.Options)
			if tc.Error != nil {
				if err == nil {
					t.Fatalf("with options %+v did not get expected error %q", tc.Options, tc.Error)
				}
				if !errors.Is(err, tc.Error) {
					t.Fatalf("with options %+v got unexpected error type %q (expected %q)", tc.Options, reflect.TypeOf(err), reflect.TypeOf(tc.Error))
				}
				return
			}
			if err != nil {
				t.Fatalf("with options %+v unexpected error %q", tc.Options, err)
			}
		})
	}
}

func TestProviderFromFlow(t *testing.T) {
	type ProviderResult struct {
		Name  string
		Flow  string
		Type  string
		Error error
	}
	tests := []ProviderResult{
		{Name: "gcr auth provider selection", Flow: gcrAuthFlow, Type: "ContainerRegistryProvider"},
		{Name: "docker-cfg auth provider selection", Flow: dockerConfigAuthFlow, Type: "DockerConfigKeyProvider"},
		{Name: "docker-cfg-url auth provider selection", Flow: dockerConfigURLAuthFlow, Type: "DockerConfigURLKeyProvider"},
		{Name: "non-existent auth provider request", Flow: "bad-flow", Type: "", Error: &AuthFlowTypeError{requestedFlow: "bad-flow"}},
		{Name: "empty auth provider request", Flow: "", Type: "", Error: &AuthFlowTypeError{requestedFlow: ""}},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			provider, err := providerFromFlow(tc.Flow, credentialproviderapi.CredentialProviderRequest{}, CredentialOptions{AuthFlow: tc.Flow})
			if tc.Error != nil {
				if err == nil {
					t.Fatalf("with flow %q did not get expected error %q", tc.Flow, err)
				}
				if !errors.Is(err, tc.Error) {
					t.Fatalf("with flow %q got unexpected error type %q (expected %q)", tc.Flow, reflect.TypeOf(err), reflect.TypeOf(tc.Error))
				}
				return
			}
			if err != nil {
				t.Fatalf("with flow %q unexpected error %q", tc.Flow, err)
			}
			providerType := reflect.TypeOf(provider).String()
			if providerType != "*gcpcredential."+tc.Type {
				t.Errorf("with flow %q unexpected provider type %q", tc.Flow, providerType)
			}
		})
	}
}

func TestFlagError(t *testing.T) {
	type FlagErrorTest struct {
		Name            string
		Options         CredentialOptions
		ExpectedError   AuthFlowFlagError
		MessageContains string
	}
	tests := []FlagErrorTest{
		{Name: "errors.Is true for different flagValues", Options: CredentialOptions{AuthFlow: "bad-flow"}, ExpectedError: AuthFlowFlagError{flagValue: "other-bad-flow"}},
		{Name: "error message contains rejected value", Options: CredentialOptions{AuthFlow: "bad-flow"}, ExpectedError: AuthFlowFlagError{flagValue: "bad-flow"}, MessageContains: "bad-flow"},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			err := validateFlags(&tc.Options)
			if !errors.Is(err, &tc.ExpectedError) {
				t.Fatalf("did not get expected error %q (got %q instead", &tc.ExpectedError, err)
			}
			if !strings.Contains(err.Error(), tc.MessageContains) {
				t.Fatalf("%q missing from error message %q", tc.MessageContains, err.Error())
			}
		})
	}
}

func TestFlowError(t *testing.T) {
	type FlowErrorTest struct {
		Name            string
		Flow            string
		ExpectedError   AuthFlowTypeError
		MessageContains string
	}
	tests := []FlowErrorTest{
		{Name: "errors.Is true for different requestedFlows", Flow: "bad-provider", ExpectedError: AuthFlowTypeError{requestedFlow: "other-bad-provider"}},
		{Name: "error message contains rejected value", Flow: "bad-provider", ExpectedError: AuthFlowTypeError{requestedFlow: "bad-provider"}, MessageContains: "bad-provider"},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			_, err := providerFromFlow(tc.Flow, credentialproviderapi.CredentialProviderRequest{}, CredentialOptions{AuthFlow: tc.Flow})
			if !errors.Is(err, &tc.ExpectedError) {
				t.Fatalf("did not get expected error %q (got %q instead", &tc.ExpectedError, err)
			}
			if !strings.Contains(err.Error(), tc.MessageContains) {
				t.Fatalf("%q missing from error message %q", tc.MessageContains, err.Error())
			}
		})
	}
}
