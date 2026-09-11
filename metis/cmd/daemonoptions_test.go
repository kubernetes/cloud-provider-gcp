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
	"testing"

	"k8s.io/metis/pkg"
	"k8s.io/metis/pkg/daemon"
)

func TestDaemonOptionsValidate(t *testing.T) {
	tests := []struct {
		name        string
		metricsPort int
		bindAddress string
		wantErr     bool
	}{
		{
			name:        "default metrics port is valid",
			metricsPort: pkg.DefaultMetricsPort,
			bindAddress: "0.0.0.0",
			wantErr:     false,
		},
		{
			name:        "metrics port 0 (disabled) is valid",
			metricsPort: 0,
			bindAddress: "0.0.0.0",
			wantErr:     false,
		},
		{
			name:        "custom valid metrics port and localhost bind address",
			metricsPort: 8080,
			bindAddress: "127.0.0.1",
			wantErr:     false,
		},
		{
			name:        "valid IPv6 bind address",
			metricsPort: 8080,
			bindAddress: "::1",
			wantErr:     false,
		},
		{
			name:        "maximum allowed port 65535 is valid",
			metricsPort: 65535,
			bindAddress: "0.0.0.0",
			wantErr:     false,
		},
		{
			name:        "negative metrics port is invalid",
			metricsPort: -1,
			bindAddress: "0.0.0.0",
			wantErr:     true,
		},
		{
			name:        "metrics port exceeding 65535 is invalid",
			metricsPort: 65536,
			bindAddress: "0.0.0.0",
			wantErr:     true,
		},
		{
			name:        "invalid bind address string is invalid",
			metricsPort: 9996,
			bindAddress: "invalid-ip",
			wantErr:     true,
		},
		{
			name:        "out of range IPv4 address is invalid",
			metricsPort: 9996,
			bindAddress: "256.256.256.256",
			wantErr:     true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opts := newDaemonOptions()
			opts.MetricsPort = tc.metricsPort
			opts.BindAddress = tc.bindAddress

			var cfg daemon.Config
			err := opts.applyTo(&cfg)
			if (err != nil) != tc.wantErr {
				t.Errorf("opts.applyTo(&cfg) error = %v, wantErr = %v", err, tc.wantErr)
			}

			if !tc.wantErr {
				if cfg.MetricsPort != tc.metricsPort {
					t.Errorf("cfg.MetricsPort = %d, want %d", cfg.MetricsPort, tc.metricsPort)
				}
				if cfg.BindAddress != tc.bindAddress {
					t.Errorf("cfg.BindAddress = %q, want %q", cfg.BindAddress, tc.bindAddress)
				}
			}
		})
	}
}
