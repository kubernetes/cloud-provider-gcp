/*
Copyright 2025 The Kubernetes Authors.
*/

package nodemanager

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/cloud-provider-gcp/pkg/apis/providerconfig/v1"
)

func TestGetClusterCIDRsFromProviderConfig(t *testing.T) {
	tests := []struct {
		name          string
		pc            *v1.ProviderConfig
		expectedCIDRs string
		expectError   bool
	}{
		{
			name: "Single Pod Range",
			pc: &v1.ProviderConfig{
				Spec: v1.ProviderConfigSpec{
					NetworkConfig: v1.ProviderNetworkConfig{
						SubnetInfo: v1.ProviderConfigSubnetInfo{
							PodRanges: []v1.ProviderConfigSecondaryRange{
								{CIDR: "10.100.0.0/16"},
							},
						},
					},
				},
			},
			expectedCIDRs: "10.100.0.0/16",
			expectError:   false,
		},
		{
			name: "Multiple Pod Ranges",
			pc: &v1.ProviderConfig{
				Spec: v1.ProviderConfigSpec{
					NetworkConfig: v1.ProviderNetworkConfig{
						SubnetInfo: v1.ProviderConfigSubnetInfo{
							PodRanges: []v1.ProviderConfigSecondaryRange{
								{CIDR: "10.100.0.0/16"},
								{CIDR: "fd00::/64"},
							},
						},
					},
				},
			},
			expectedCIDRs: "10.100.0.0/16,fd00::/64",
			expectError:   false,
		},
		{
			name: "Empty Pod Ranges List",
			pc: &v1.ProviderConfig{
				Spec: v1.ProviderConfigSpec{
					NetworkConfig: v1.ProviderNetworkConfig{
						SubnetInfo: v1.ProviderConfigSubnetInfo{
							PodRanges: []v1.ProviderConfigSecondaryRange{},
						},
					},
				},
			},
			expectedCIDRs: "",
			expectError:   true,
		},
		{
			name: "Pod Range with Empty CIDR",
			pc: &v1.ProviderConfig{
				Spec: v1.ProviderConfigSpec{
					NetworkConfig: v1.ProviderNetworkConfig{
						SubnetInfo: v1.ProviderConfigSubnetInfo{
							PodRanges: []v1.ProviderConfigSecondaryRange{
								{CIDR: ""},
							},
						},
					},
				},
			},
			expectedCIDRs: "",
			expectError:   true,
		},
		{
			name: "Mixed Valid and Empty CIDR",
			pc: &v1.ProviderConfig{
				Spec: v1.ProviderConfigSpec{
					NetworkConfig: v1.ProviderNetworkConfig{
						SubnetInfo: v1.ProviderConfigSubnetInfo{
							PodRanges: []v1.ProviderConfigSecondaryRange{
								{CIDR: "10.100.0.0/16"},
								{CIDR: ""},
							},
						},
					},
				},
			},
			expectedCIDRs: "10.100.0.0/16",
			expectError:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cidrs, err := getClusterCIDRsFromProviderConfig(tc.pc)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedCIDRs, cidrs)
			}
		})
	}
}
