/*
Copyright 2025 The Kubernetes Authors.
*/

package gketenantcontrollers

import (
	"testing"

	v1 "github.com/GoogleCloudPlatform/gke-enterprise-mt/apis/providerconfig/v1"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestTokenURLForProviderConfig(t *testing.T) {
	tests := []struct {
		name             string
		existingTokenURL string
		providerConfig   *v1.ProviderConfig
		expectedTokenURL string
		expectError      bool
	}{
		{
			name:             "Standard GKE Token URL",
			existingTokenURL: "https://gkeauth.googleapis.com/v1/projects/my-project/locations/us-central1/clusters/my-cluster:generateToken",
			providerConfig: &v1.ProviderConfig{
				Spec: v1.ProviderConfigSpec{
					ProjectNumber: 123456789,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-tenant-name",
					Labels: map[string]string{
						tenantLabel: "true",
					},
				},
			},
			// Base: https://gkeauth.googleapis.com/v1
			// Project: 123456789
			// Location: us-central1
			// Tenant: my-tenant-name
			expectedTokenURL: "https://gkeauth.googleapis.com/v1/projects/123456789/locations/us-central1/tenants/my-tenant-name:generateTenantToken",
			expectError:      false,
		},
		{
			name:             "No Tenant Label",
			existingTokenURL: "https://gkeauth.googleapis.com/v1/projects/my-project/locations/us-central1/clusters/my-cluster:generateToken",
			providerConfig: &v1.ProviderConfig{
				Spec: v1.ProviderConfigSpec{
					ProjectNumber: 123456789,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:   "my-tenant-name",
					Labels: map[string]string{}, // Empty labels
				},
			},
			expectedTokenURL: "https://gkeauth.googleapis.com/v1/projects/my-project/locations/us-central1/clusters/my-cluster:generateToken",
			expectError:      false,
		},
		{
			name:             "Invalid Token URL (No /projects/)",
			existingTokenURL: "https://gkeauth.googleapis.com/v1/no-projects/my-project/locations/us-central1",
			providerConfig: &v1.ProviderConfig{
				Spec: v1.ProviderConfigSpec{
					ProjectNumber: 123456789,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-tenant-name",
					Labels: map[string]string{
						tenantLabel: "true",
					},
				},
			},
			expectedTokenURL: "",
			expectError:      true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tokenURL, err := tokenURLForProviderConfig(tc.existingTokenURL, tc.providerConfig)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedTokenURL, tokenURL)
			}
		})
	}
}
