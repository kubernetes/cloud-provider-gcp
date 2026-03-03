/*
Copyright 2026 The Kubernetes Authors.
*/

package gketenantcontrollers

import (
	"regexp"
	"testing"

	v1 "github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/apis/providerconfig/v1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gce "k8s.io/cloud-provider-gcp/providers/gce"
)

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name           string
		providerConfig *v1.ProviderConfig
		expectError    bool
	}{
		{
			name: "Valid Config Full",
			providerConfig: &v1.ProviderConfig{
				Spec: v1.ProviderConfigSpec{
					NetworkConfig: v1.ProviderNetworkConfig{
						Network: "projects/my-project/global/networks/my-network",
						SubnetInfo: v1.ProviderConfigSubnetInfo{
							Subnetwork: "projects/my-project/regions/us-central1/subnetworks/my-subnet",
						},
					},
					AuthConfig: &v1.AuthConfig{
						TokenURL:  "https://gkeauth.googleapis.com/v1/projects/123/locations/us-central1/tenants/my-tenant:generateTenantToken",
						TokenBody: "{}",
					},
				},
			},
			expectError: false,
		},
		{
			name: "Valid Config Non-Prod",
			providerConfig: &v1.ProviderConfig{
				Spec: v1.ProviderConfigSpec{
					AuthConfig: &v1.AuthConfig{
						TokenURL:  "https://staging-gkeauth.sandbox.googleapis.com/v1/projects/123/locations/us-central1/tenants/my-tenant:generateTenantToken",
						TokenBody: "{}",
					},
				},
			},
			expectError: false,
		},
		{
			name: "Valid Config Cluster Token",
			providerConfig: &v1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"tenancy.gke.io/access-level": "supervisor",
					},
				},
				Spec: v1.ProviderConfigSpec{
					AuthConfig: &v1.AuthConfig{
						TokenURL:  "https://gkeauth.googleapis.com/v1/projects/654321/locations/us-central1/clusters/example-cluster:generateToken",
						TokenBody: "{}",
					},
				},
			},
			expectError: false,
		},
		{
			name: "Supervisor Config with Tenant Token",
			providerConfig: &v1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"tenancy.gke.io/access-level": "supervisor",
					},
				},
				Spec: v1.ProviderConfigSpec{
					AuthConfig: &v1.AuthConfig{
						TokenURL:  "https://gkeauth.googleapis.com/v1/projects/123/locations/us-central1/tenants/my-tenant:generateTenantToken",
						TokenBody: "{}",
					},
				},
			},
			expectError: true,
		},
		{
			name: "Invalid Network URL",
			providerConfig: &v1.ProviderConfig{
				Spec: v1.ProviderConfigSpec{
					NetworkConfig: v1.ProviderNetworkConfig{
						Network: "my-network",
					},
				},
			},
			expectError: true,
		},
		{
			name: "Invalid Subnetwork URL",
			providerConfig: &v1.ProviderConfig{
				Spec: v1.ProviderConfigSpec{
					NetworkConfig: v1.ProviderNetworkConfig{
						SubnetInfo: v1.ProviderConfigSubnetInfo{
							Subnetwork: "my-subnet",
						},
					},
				},
			},
			expectError: true,
		},
		{
			name: "Invalid Token URL",
			providerConfig: &v1.ProviderConfig{
				Spec: v1.ProviderConfigSpec{
					AuthConfig: &v1.AuthConfig{
						TokenURL:  "https://dummy.googleapis.com/token",
						TokenBody: "{}",
					},
				},
			},
			expectError: true,
		},
		{
			name: "Empty TokenBody",
			providerConfig: &v1.ProviderConfig{
				Spec: v1.ProviderConfigSpec{
					AuthConfig: &v1.AuthConfig{
						TokenURL:  "https://gkeauth.googleapis.com/token",
						TokenBody: "",
					},
				},
			},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateConfig(tc.providerConfig)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSetNetworkConfig(t *testing.T) {
	tests := []struct {
		name        string
		network     string
		expectError bool
		wantName    string
		wantURL     string
	}{
		{
			name:        "Valid Network URL",
			network:     "projects/my-project/global/networks/my-network",
			expectError: false,
			wantName:    "my-network",
			wantURL:     "projects/my-project/global/networks/my-network",
		},
		{
			name:        "Invalid Network URL",
			network:     "invalid-url",
			expectError: true,
		},
		{
			name:        "Empty Network URL",
			network:     "",
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := &gce.CloudConfig{}
			err := setNetworkConfig(c, tc.network)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.wantName, c.NetworkName)
				assert.Equal(t, tc.wantURL, c.NetworkURL)
			}
		})
	}
}

func TestSetSubnetworkConfig(t *testing.T) {
	tests := []struct {
		name        string
		subnetwork  string
		expectError bool
		wantName    string
		wantURL     string
	}{
		{
			name:        "Valid Subnetwork URL",
			subnetwork:  "projects/my-project/regions/us-central1/subnetworks/my-subnet",
			expectError: false,
			wantName:    "my-subnet",
			wantURL:     "projects/my-project/regions/us-central1/subnetworks/my-subnet",
		},
		{
			name:        "Invalid Subnetwork URL",
			subnetwork:  "invalid-url",
			expectError: true,
		},
		{
			name:        "Empty Subnetwork URL",
			subnetwork:  "",
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := &gce.CloudConfig{}
			err := setSubnetworkConfig(c, tc.subnetwork)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.wantName, c.SubnetworkName)
				assert.Equal(t, tc.wantURL, c.SubnetworkURL)
			}
		})
	}
}

func TestValidateField(t *testing.T) {
	regex := regexp.MustCompile(`^[a-z]+$`)
	tests := []struct {
		name        string
		fieldName   string
		value       string
		pattern     *regexp.Regexp
		expectError bool
	}{
		{
			name:        "Valid Match",
			fieldName:   "testField",
			value:       "abc",
			pattern:     regex,
			expectError: false,
		},
		{
			name:        "Invalid Match",
			fieldName:   "testField",
			value:       "123",
			pattern:     regex,
			expectError: true,
		},
		{
			name:        "Empty Value",
			fieldName:   "testField",
			value:       "",
			pattern:     regex,
			expectError: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateField(tc.fieldName, tc.value, tc.pattern)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestURLRegexes(t *testing.T) {
	tests := []struct {
		name    string
		pattern *regexp.Regexp
		match   []string
		noMatch []string
	}{
		{
			name:    "networkURLRegex",
			pattern: networkURLRegex,
			match: []string{
				"projects/my-project/global/networks/my-network",
				"projects/123/global/networks/abc-def",
			},
			noMatch: []string{
				"projects/my-project/global/networks/",
				"projects/my-project/global/networks/my-network/extra",
				"projects/my-project/regions/us-central1/networks/my-network",
				"my-network",
				"",
			},
		},
		{
			name:    "subnetworkURLRegex",
			pattern: subnetworkURLRegex,
			match: []string{
				"projects/my-project/regions/us-central1/subnetworks/my-subnet",
				"projects/123/regions/europe-west1/subnetworks/abc-def",
			},
			noMatch: []string{
				"projects/my-project/regions/us-central1/subnetworks/",
				"projects/my-project/regions/us-central1/subnetworks/my-subnet/extra",
				"projects/my-project/global/subnetworks/my-subnet",
				"my-subnet",
				"",
			},
		},
		{
			name:    "tenantTokenURLRegex",
			pattern: tenantTokenURLRegex,
			match: []string{
				"https://gkeauth.googleapis.com/v1/projects/123/locations/us-central1/tenants/my-tenant:generateTenantToken",
				"https://staging-gkeauth.sandbox.googleapis.com/v1/projects/123/locations/us-central1/tenants/my-tenant:generateTenantToken",
				"https://gkeauth.googleapis.com/projects/123/locations/us-central1/tenants/my-tenant:generateTenantToken",
				"https://gkeauth.sandbox.googleapis.com/v1/projects/123/locations/us-central1/tenants/my-tenant:generateTenantToken",
			},
			noMatch: []string{
				"https://dummy.googleapis.com/v1/projects/123/locations/us-central1/tenants/my-tenant:generateTenantToken",
				"https://gkeauth.googleapis.com/v1/projects/abc/locations/us-central1/tenants/my-tenant:generateTenantToken",
				"https://gkeauth.googleapis.com/v1/projects/123/locations/us-central1/tenants/my-tenant:generateToken",
				"http://gkeauth.googleapis.com/v1/projects/123/locations/us-central1/tenants/my-tenant:generateTenantToken",
				"https://gkeauth.googleapis.com/v1/projects/123/locations/us-central1/tenants/my-tenant:generateTenantToken/extra",
				"",
			},
		},
		{
			name:    "clusterTokenURLRegex",
			pattern: clusterTokenURLRegex,
			match: []string{
				"https://gkeauth.googleapis.com/v1/projects/123/locations/us-central1/clusters/example-cluster:generateToken",
				"https://staging-gkeauth.sandbox.googleapis.com/v1/projects/654321/locations/us-central1/clusters/example-cluster:generateToken",
				"https://gkeauth.googleapis.com/projects/123/locations/us-central1/clusters/example-cluster:generateToken",
				"https://gkeauth.sandbox.googleapis.com/v1/projects/123/locations/us-central1/clusters/example-cluster:generateToken",
			},
			noMatch: []string{
				"https://dummy.googleapis.com/v1/projects/123/locations/us-central1/clusters/example-cluster:generateToken",
				"https://gkeauth.googleapis.com/v1/projects/abc/locations/us-central1/clusters/example-cluster:generateToken",
				"https://gkeauth.googleapis.com/v1/projects/123/locations/us-central1/clusters/example-cluster:generateTenantToken",
				"http://gkeauth.googleapis.com/v1/projects/123/locations/us-central1/clusters/example-cluster:generateToken",
				"https://gkeauth.googleapis.com/v1/projects/123/locations/us-central1/clusters/example-cluster:generateToken/extra",
				"",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, m := range tc.match {
				t.Run("Match:"+m, func(t *testing.T) {
					assert.True(t, tc.pattern.MatchString(m), "expected %q to match %s", m, tc.name)
				})
			}
			for _, n := range tc.noMatch {
				t.Run("NoMatch:"+n, func(t *testing.T) {
					assert.False(t, tc.pattern.MatchString(n), "expected %q to not match %s", n, tc.name)
				})
			}
		})
	}
}
