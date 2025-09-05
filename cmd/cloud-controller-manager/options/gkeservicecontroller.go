/*
Copyright 2025 The Kubernetes Authors.

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

package options

import (
	"github.com/spf13/pflag"
)

// GkeServiceControllerOptions holds the GkeServiceController options.
type GkeServiceControllerOptions struct {
	// EnableGkeServiceController is bound to a command-line flag. When true, it
	// enables the GKE implementation of the service controller.
	// This is an opt-in for projects that use GKE features.
	EnableGkeServiceController bool
}

// AddFlags adds flags related to GkeServiceController for controller manager to the specified FlagSet.
func (o *GkeServiceControllerOptions) AddFlags(fs *pflag.FlagSet) {
	if o == nil {
		return
	}
	fs.BoolVar(&o.EnableGkeServiceController, "enable-gke-service-controller", false, "Enables the GKE specific implementation of the service controller. This is intended for projects that use GKE features and is disabled by default to avoid overriding the standard controller.")
}

// ApplyTo is a no-op for GkeServiceControllerOptions.
func (o *GkeServiceControllerOptions) ApplyTo() error {
	return nil
}

// Validate checks validation of GkeServiceControllerOptions.
func (o *GkeServiceControllerOptions) Validate() []error {
	return nil
}
