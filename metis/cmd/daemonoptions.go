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
	"time"

	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/metis/daemon"
)

// DaemonOptions holds the metis daemon options.
type DaemonOptions struct {
	*daemon.Config
}

// AddFlags returns flags for the metis daemon options by section name.
func (o *DaemonOptions) AddFlags() cliflag.NamedFlagSets {
	fss := cliflag.NamedFlagSets{}
	if o == nil {
		return fss
	}

	fs := fss.FlagSet("daemon")
	// We apply default values directly within Flags for now, or assume they are pre-initialized
	fs.DurationVar(&o.MonitorInterval, "monitor-interval", 5*time.Second, "Monitor interval (e.g., 5s, 1m)")
	fs.DurationVar(&o.ReleaseCooldown, "release-cooldown", 1*time.Minute, "Release cooldown duration (e.g., 5m)")

	return fss
}

// ApplyTo fills up the daemon config with options.
func (o *DaemonOptions) ApplyTo(cfg *daemon.Config) error {
	if o == nil || cfg == nil {
		return nil
	}

	cfg.MonitorInterval = o.MonitorInterval
	cfg.ReleaseCooldown = o.ReleaseCooldown

	return nil
}
