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
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/metis/pkg"
	"k8s.io/metis/pkg/daemon"
)

// daemonOptions holds the metis daemon options.
type daemonOptions struct {
	*daemon.Config
}

// newDaemonOptions returns a new daemonOptions instance with default configurations.
func newDaemonOptions() *daemonOptions {
	return &daemonOptions{
		Config: &daemon.Config{},
	}
}

// addFlags returns flags for the metis daemon options by section name.
func (o *daemonOptions) addFlags() cliflag.NamedFlagSets {
	fss := cliflag.NamedFlagSets{}
	if o == nil {
		return fss
	}

	fs := fss.FlagSet("daemon")
	fs.DurationVar(&o.MonitorInterval, "monitor-interval", daemon.DefaultMonitorInterval, "Monitor interval (e.g., 5s, 1m). 0 or negative values will be interpreted as the default value.")
	fs.DurationVar(&o.ReleaseCooldown, "release-cooldown", daemon.DefaultReleaseCooldown, "Release cooldown duration (e.g., 5m). 0 or negative values will be interpreted as the default value.")
	fs.StringVar(&o.DBPath, "db-path", pkg.DefaultDBPath, "Path to the SQLite database file")
	fs.StringVar(&o.SocketPath, "socket-path", pkg.DefaultSockPath, "Path to the Unix domain socket")
	fs.DurationVar(&o.DrainingExpiration, "draining-expiration", daemon.DefaultDrainingExpiration, "Draining expiration duration (e.g., 5h). 0 or negative values will be interpreted as the default value.")
	fs.DurationVar(&o.SustainedLowUtilizationDuration, "sustained-low-utilization-duration", daemon.DefaultSustainedLowUtilizationDuration, "Sustained low utilization duration (e.g., 8h). 0 or negative values will be interpreted as the default value.")

	return fss
}

// applyTo fills up the daemon config with options.
func (o *daemonOptions) applyTo(cfg *daemon.Config) error {
	if o == nil || cfg == nil {
		return nil
	}

	cfg.MonitorInterval = o.MonitorInterval
	cfg.ReleaseCooldown = o.ReleaseCooldown
	cfg.DBPath = o.DBPath
	cfg.SocketPath = o.SocketPath
	cfg.DrainingExpiration = o.DrainingExpiration
	cfg.SustainedLowUtilizationDuration = o.SustainedLowUtilizationDuration

	return nil
}
