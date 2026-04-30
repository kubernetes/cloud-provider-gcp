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

package cni

import (
	"net"
	"testing"
)

func TestLoadNetConf(t *testing.T) {
	tests := []struct {
		name        string
		input       []byte
		expectError bool
		validate    func(t *testing.T, conf *PluginConf)
	}{
		{
			name:  "Valid simple config",
			input: []byte(`{"cniVersion": "0.4.0", "name": "test-net", "type": "metis", "ipam": {"type": "metis"}}`),
			validate: func(t *testing.T, conf *PluginConf) {
				if conf.CNIVersion != "0.4.0" {
					t.Errorf("expected cniVersion 0.4.0, got %s", conf.CNIVersion)
				}
				if conf.Name != "test-net" {
					t.Errorf("expected name test-net, got %s", conf.Name)
				}
			},
		},
		{
			name: "Valid dual-stack config",
			input: []byte(`{
				"name": "gke-pod-network",
				"cniVersion": "0.3.1",
				"type": "gke",
				"ipam": {
					"type": "metis",
					"ranges": [
						[{"subnet":"10.160.7.0/24"}],[{"subnet":"2600:1900:4040:ae7:0:7::/112"}]
					],
					"routes": [
						{"dst": "0.0.0.0/0"},{"dst": "::/0"}
					]
				}
			}`),
			validate: func(t *testing.T, conf *PluginConf) {
				if conf.Name != "gke-pod-network" {
					t.Errorf("expected gke-pod-network, got %s", conf.Name)
				}
				if len(conf.IPAM.Ranges) != 2 {
					t.Errorf("expected 2 subnet ranges, got %d", len(conf.IPAM.Ranges))
				}
				if len(conf.IPAM.Routes) != 2 {
					t.Errorf("expected 2 routes, got %d", len(conf.IPAM.Routes))
				}
			},
		},
		{
			name: "Valid single stack IPv4 config",
			input: []byte(`{
				"name": "v4-network",
				"cniVersion": "0.4.0",
				"type": "metis",
				"ipam": {
					"type": "metis",
					"ranges": [[{"subnet":"10.160.7.0/24"}]],
					"routes": [{"dst": "0.0.0.0/0"}]
				}
			}`),
			validate: func(t *testing.T, conf *PluginConf) {
				if conf.Name != "v4-network" {
					t.Errorf("expected v4-network, got %s", conf.Name)
				}
				if len(conf.IPAM.Ranges) != 1 {
					t.Errorf("expected 1 subnet range, got %d", len(conf.IPAM.Ranges))
				}
				if len(conf.IPAM.Routes) != 1 {
					t.Errorf("expected 1 route, got %d", len(conf.IPAM.Routes))
				}
			},
		},
		{
			name: "Valid single stack IPv6 config",
			input: []byte(`{
				"name": "v6-network",
				"cniVersion": "0.4.0",
				"type": "metis",
				"ipam": {
					"type": "metis",
					"ranges": [[{"subnet":"2600:1900:4040:ae7:0:7::/112"}]],
					"routes": [{"dst": "::/0"}]
				}
			}`),
			validate: func(t *testing.T, conf *PluginConf) {
				if conf.Name != "v6-network" {
					t.Errorf("expected v6-network, got %s", conf.Name)
				}
				if len(conf.IPAM.Ranges) != 1 {
					t.Errorf("expected 1 subnet range, got %d", len(conf.IPAM.Ranges))
				}
				if len(conf.IPAM.Routes) != 1 {
					t.Errorf("expected 1 route, got %d", len(conf.IPAM.Routes))
				}
			},
		},
		{
			name: "Valid daemon socket and log file overrides",
			input: []byte(`{
				"name": "test-overrides",
				"cniVersion": "0.4.0",
				"type": "metis",
				"daemonSocket": "/var/run/metis-test.sock",
				"logFile": "/var/log/metis-test.log",
				"ipam": {"type": "metis"}
			}`),
			validate: func(t *testing.T, conf *PluginConf) {
				if conf.DaemonSocket != "/var/run/metis-test.sock" {
					t.Errorf("expected custom socket, got %s", conf.DaemonSocket)
				}
				if conf.LogFile != "/var/log/metis-test.log" {
					t.Errorf("expected custom logFile, got %s", conf.LogFile)
				}
			},
		},
		{
			name:        "Invalid JSON string",
			input:       []byte(`{invalid json string}`),
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			conf, err := loadNetConf(tc.input)
			if tc.expectError {
				if err == nil {
					t.Errorf("expected error but got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.validate != nil {
				tc.validate(t, conf)
			}
		})
	}
}

func TestGetGatewayIP(t *testing.T) {
	tests := []struct {
		name       string
		subnet     string
		expectedGW string
	}{
		{
			name:       "Normal IPv4 CIDR",
			subnet:     "10.240.0.0/24",
			expectedGW: "10.240.0.1",
		},
		{
			name:       "Normal IPv6 CIDR",
			subnet:     "2600:1900:4040:ae7:0:7::/112",
			expectedGW: "2600:1900:4040:ae7:0:7::1",
		},
		{
			name:       "Small IPv4 CIDR",
			subnet:     "10.240.0.0/31",
			expectedGW: "169.254.1.1",
		},
		{
			name:       "Small IPv6 CIDR",
			subnet:     "2600:1900:4040:ae7:0:7::/128",
			expectedGW: "fe80::1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, ipNet, err := net.ParseCIDR(tc.subnet)
			if err != nil {
				t.Fatalf("failed to parse test subnet %s: %v", tc.subnet, err)
			}
			gw := getGatewayIP(ipNet)
			if !gw.Equal(net.ParseIP(tc.expectedGW)) {
				t.Errorf("expected gateway %s, got %v", tc.expectedGW, gw)
			}
		})
	}
}

func TestLoadK8sArgs(t *testing.T) {
	args := "K8S_POD_NAME=test-pod;K8S_POD_NAMESPACE=test-ns"
	k8sArgs, err := loadK8sArgs(args)
	if err != nil {
		t.Fatalf("loadK8sArgs failed: %v", err)
	}

	if string(k8sArgs.K8S_POD_NAME) != "test-pod" {
		t.Errorf("expected K8S_POD_NAME test-pod, got %s", k8sArgs.K8S_POD_NAME)
	}
	if string(k8sArgs.K8S_POD_NAMESPACE) != "test-ns" {
		t.Errorf("expected K8S_POD_NAMESPACE test-ns, got %s", k8sArgs.K8S_POD_NAMESPACE)
	}

	invalidArgs := "invalid;format"
	_, err = loadK8sArgs(invalidArgs)
	if err == nil {
		t.Errorf("expected error with invalid format, got nil")
	}
}
