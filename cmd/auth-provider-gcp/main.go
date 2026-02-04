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
	"flag"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/cloud-provider-gcp/cmd/auth-provider-gcp/app"
	klog "k8s.io/klog/v2"
)

func main() {
	defer klog.Flush()
	rootCmd := &cobra.Command{
		Use:   "auth-provider-gcp",
		Short: "GCP CRI authentication plugin",
	}
	credCmd, err := app.NewGetCredentialsCommand()
	if err != nil {
		klog.Errorf("%s", err.Error())
		os.Exit(1)
	}
	rootCmd.AddCommand(credCmd)
	klog.InitFlags(nil)
	flag.Parse()
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	if err := rootCmd.Execute(); err != nil {
		klog.Errorf("%s", err.Error())
		os.Exit(1)
	}
}
