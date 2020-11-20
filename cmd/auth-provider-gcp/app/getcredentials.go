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
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/cloud-provider-gcp/cmd/auth-provider-gcp/provider"
	"k8s.io/cloud-provider-gcp/pkg/credentialconfig"
	klog "k8s.io/klog/v2"
	"net/http"
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
	if err := validateFlags(); err != nil {
		return nil, err
	}
	return cmd, nil
}

func getCredentials(cmd *cobra.Command, args []string) error {
	klog.V(2).Infof("get-credentials %s", authFlow)
	transport := utilnet.SetTransportDefaults(&http.Transport{})
	var authProvider credentialconfig.DockerConfigProvider
	switch authFlow {
	case gcrAuthFlow:
		authProvider = provider.MakeRegistryProvider(transport)
	case dockerConfigAuthFlow:
		authProvider = provider.MakeDockerConfigProvider(transport)
	case dockerConfigURLAuthFlow:
		authProvider = provider.MakeDockerConfigURLProvider(transport)
	default:
		return fmt.Errorf("unrecognized auth flow \"%s\"", authFlow)
	}
	authCredentials, err := provider.GetResponse(authProvider)
	if err != nil {
		return err
	}
	jsonResponse, err := json.Marshal(authCredentials)
	if err != nil {
		return err
	}
	// Emit authentication request for kubelet to consume 
	fmt.Println(string(jsonResponse))
	return nil
}

func defineFlags(credCmd *cobra.Command) {
	credCmd.Flags().StringVarP(&authFlow, "authFlow", "a", gcrAuthFlow, "authentication flow (valid values are gcr, dockercfg, and dockercfg-url)")
}

func validateFlags() error {
	if authFlow != gcrAuthFlow && authFlow != dockerConfigAuthFlow && authFlow != dockerConfigURLAuthFlow {
		return fmt.Errorf("invalid value \"%s\" for authFlow (must be one of \"%s\", \"%s\", or \"%s\")", authFlow, gcrAuthFlow, dockerConfigAuthFlow, dockerConfigURLAuthFlow)
	}
	return nil
}
