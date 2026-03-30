/*
Copyright 2026 The Kubernetes Authors.

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

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"k8s.io/component-base/logs"
	"k8s.io/klog/v2"
)

func main() {
	defer logs.FlushLogs()
	klog.InitFlags(nil)
	logs.InitLogs()

	rootCmd := &cobra.Command{
		Use:   "metis",
		Short: "Metis implements adaptive cluster IPAM for GKE",
		Run: func(cmd *cobra.Command, args []string) {
			klog.InfoS("metis started in non-daemon mode")
			// TODO: fix me for CNI mode
		},
	}

	rootCmd.AddCommand(newDaemonCommand())

	flag.Parse()
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)

	if err := rootCmd.Execute(); err != nil {
		klog.ErrorS(err, "metis command failed")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}
}
