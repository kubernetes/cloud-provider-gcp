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
	"k8s.io/cloud-provider-gcp/cmd/auth-provider-gcp/plugin"
)

var (
	metadataUrl        string
	storageScopePrefix string
	cloudPlatformScope string
)

func NewGetCredentialsCommand() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "get-credentials",
		Short: "Get credentials for a container image",
		RunE: func(cmd *cobra.Command, args []string) error {
			// TODO(DangerOnTheRanger): don't use hardcoded image name
			image := "hello-world"
			authCredentials, err := plugin.GetResponse(image, metadataUrl, storageScopePrefix, cloudPlatformScope)
			if err != nil {
				return err
			}
			jsonResponse, err := json.Marshal(authCredentials)
			if err != nil {
				return err
			}
			fmt.Println(string(jsonResponse))
			return nil
		},
	}
	defineFlags(cmd)
	if err := validateFlags(cmd); err != nil {
		return nil, err
	}
	return cmd, nil
}

func defineFlags(credCmd *cobra.Command) {
	credCmd.Flags().StringVarP(&metadataUrl, "metadataUrl", "", "", "metadata URL (required)")
	credCmd.MarkFlagRequired("metadataUrl")
	credCmd.Flags().StringVarP(&storageScopePrefix, "storageScopePrefix", "", "", "storage scope prefix (required)")
	credCmd.MarkFlagRequired("storageScopePrefix")
	credCmd.Flags().StringVarP(&cloudPlatformScope, "cloudPlatformScope", "", "", "cloud platform scope (required)")
	credCmd.MarkFlagRequired("cloudPlatformScope")
}

func validateFlags(credCmd *cobra.Command) error {
	// TODO (DangerOnTheRanger): add appropriate flag validation
	return nil
}
