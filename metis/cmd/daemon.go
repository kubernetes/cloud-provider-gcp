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
	"os"

	"github.com/spf13/cobra"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/klog/v2"
	"k8s.io/metis/daemon"
)

func newDaemonCommand() *cobra.Command {

	opts := &DaemonOptions{
		Config: &daemon.Config{},
	}

	// Define command-line flags to configure the daemon
	fss := opts.AddFlags()

	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Run the metis daemon",
		Run: func(cmd *cobra.Command, args []string) {
			cliflag.PrintFlags(cmd.Flags())
			var cfg daemon.Config
			_ = opts.ApplyTo(&cfg)
			d := daemon.NewDaemon(cfg)
			if err := d.Run(); err != nil {
				klog.ErrorS(err, "Daemon failed to run")
				os.Exit(1)
			}
		},
	}

	for _, f := range fss.FlagSets {
		cmd.Flags().AddFlagSet(f)
	}

	return cmd
}
