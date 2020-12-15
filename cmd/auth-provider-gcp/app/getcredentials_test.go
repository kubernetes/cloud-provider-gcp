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
	"fmt"
	"reflect"
	"testing"
)

func flagError(val string) error {
	return fmt.Errorf("invalid value %q for authFlow (must be one of %q, %q, or %q)", val, gcrAuthFlow, dockerConfigAuthFlow, dockerConfigURLAuthFlow)
}

func TestValidateAuthFlow(t *testing.T) {
	type FlagResult struct {
		Flow  string
		Error error
	}
	tests := []FlagResult{
		{Flow: gcrAuthFlow, Error: nil},
		{Flow: dockerConfigAuthFlow, Error: nil},
		{Flow: dockerConfigURLAuthFlow, Error: nil},
		{Flow: "bad-flow", Error: &AuthFlowFlagError{flagValue: "bad-flow"}},
	}
	for _, tc := range tests {
		err := validateFlags(tc.Flow)
		if err != nil && tc.Error == nil {
			t.Errorf("with flow %q unexpected error %q", tc.Flow, err)
		}
		if err == nil && tc.Error != nil {
			t.Errorf("with flow %q did not get expected error %q", tc.Flow, err)
		}
		if err != nil && tc.Error != nil {
			if reflect.TypeOf(err) != reflect.TypeOf(tc.Error) {
				t.Errorf("with flow %q got unexpected error type %q (expected %q)", tc.Flow, reflect.TypeOf(err), reflect.TypeOf(tc.Error))
			}
		}
	}
}

func providerError(val string) error {
	return fmt.Errorf("unrecognized auth flow \"%s\"", val)
}

func TestProviderFromFlow(t *testing.T) {
	type ProviderResult struct {
		Flow  string
		Type  string
		Error error
	}
	tests := []ProviderResult{
		{Flow: gcrAuthFlow, Type: "ContainerRegistryProvider", Error: nil},
		{Flow: dockerConfigAuthFlow, Type: "DockerConfigKeyProvider", Error: nil},
		{Flow: dockerConfigURLAuthFlow, Type: "DockerConfigURLKeyProvider", Error: nil},
		{Flow: "bad-flow", Type: "", Error: &AuthFlowTypeError{requestedFlow: "bad-flow"}},
	}
	for _, tc := range tests {
		provider, err := providerFromFlow(tc.Flow)
		if err != nil && tc.Error == nil {
			t.Errorf("with flow %q unexpected error %q", tc.Flow, err)
		}
		if err == nil && tc.Error != nil {
			t.Errorf("with flow %q did not get expected error %q", tc.Flow, err)
		}
		if err != nil && tc.Error != nil {
			if reflect.TypeOf(err) != reflect.TypeOf(tc.Error) {
				t.Errorf("with flow %q got unexpected error type %q (expected %q)", tc.Flow, reflect.TypeOf(err), reflect.TypeOf(tc.Error))
			}
		}
		if tc.Type == "" && provider != nil {
			t.Errorf("with flow %q got unexpectedly non-nil provider %q", provider)
		}
		// The nil check is meant for test cases where provider is nil on purpose,
		// i,e, for error cases - any errors will get tested and caught above, so in
		// those cases, we don't need to run additional checks
		if provider != nil {
			providerType := reflect.TypeOf(provider).String()
			if providerType != "*gcpcredential."+tc.Type {
				t.Errorf("with flow %q unexpected provider type %q", tc.Flow, providerType)
			}
		}
	}
}
