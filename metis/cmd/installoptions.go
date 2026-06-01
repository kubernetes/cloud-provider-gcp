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

	"github.com/spf13/pflag"
)

// installOptions holds the CLI options for the self-installer.
type installOptions struct {
	CniDir string
}

// newInstallOptions returns options with default values.
func newInstallOptions() *installOptions {
	return &installOptions{
		CniDir: os.Getenv("CNI_DIR"),
	}
}

// addFlags registers installer flags to the specified FlagSet.
func (o *installOptions) addFlags(fs *pflag.FlagSet) {
	if o == nil {
		return
	}
	fs.StringVar(&o.CniDir, "cni-dir", o.CniDir, "CNI directory path (falls back to CNI_DIR env, then /host/opt/cni)")
}

// validate verifies the parsed options.
func (o *installOptions) validate() error {
	if o.CniDir == "" {
		o.CniDir = "/host/opt/cni"
	}
	return nil
}
