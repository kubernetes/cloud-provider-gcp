/*
Copyright 2026 The Kubernetes Authors.
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

func TestExtractLocationFromTokenURL(t *testing.T) {
	tests := []struct {
		name     string
		tokenURL string
		want     string
	}{
		{
			name:     "Valid URL",
			tokenURL: "https://gkeauth.googleapis.com/v1/projects/my-project/locations/us-central1/clusters/my-cluster:generateToken",
			want:     "us-central1",
		},
		{
			name:     "No Location",
			tokenURL: "https://gkeauth.googleapis.com/v1/projects/my-project",
			want:     "",
		},
		{
			name:     "Empty URL",
			tokenURL: "",
			want:     "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractLocationFromTokenURL(tc.tokenURL)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestUpdateTokenProjectNumber(t *testing.T) {
	tests := []struct {
		name          string
		tokenBody     string
		projectNumber int
		want          string
		wantErr       bool
	}{
		{
			name:          "Valid JSON",
			tokenBody:     `{"aud":"https://kubernetes.io/","projectNumber":123}`,
			projectNumber: 456,
			want:          `{"aud":"https://kubernetes.io/","projectNumber":456}`,
			wantErr:       false,
		},
		{
			name:          "Quoted JSON",
			tokenBody:     `"{\"aud\":\"https://kubernetes.io/\",\"projectNumber\":123}"`,
			projectNumber: 456,
			want:          `{"aud":"https://kubernetes.io/","projectNumber":456}`,
			wantErr:       false,
		},
		{
			name:          "Invalid JSON",
			tokenBody:     `{invalid-json}`,
			projectNumber: 456,
			want:          "",
			wantErr:       true,
		},
		{
			name:          "Empty Body",
			tokenBody:     "",
			projectNumber: 456,
			want:          "",
			wantErr:       false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := updateTokenProjectNumber(tc.tokenBody, tc.projectNumber)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tc.want == "" {
					assert.Equal(t, tc.want, got)
				} else {
					assert.JSONEq(t, tc.want, got)
				}
			}
		})
	}
}

func TestGetNodeLabelSelector(t *testing.T) {
	tests := []struct {
		name           string
		providerConfig *v1.ProviderConfig
		want           string
		wantErr        bool
	}{
		{
			name: "Valid Config",
			providerConfig: &v1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: "config1",
				},
			},
			want:    "config1",
			wantErr: false,
		},
		{
			name: "Empty Name",
			providerConfig: &v1.ProviderConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: "",
				},
			},
			want:    "",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := getNodeLabelSelector(tc.providerConfig)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.want, got)
			}
		})
	}
}

func TestParseResourceURL(t *testing.T) {
	tests := []struct {
		name         string
		resource     string
		expectedName string
		expectedURL  string
	}{
		{
			name:         "Full URL",
			resource:     "projects/my-project/global/networks/my-network",
			expectedName: "my-network",
			expectedURL:  "projects/my-project/global/networks/my-network",
		},
		{
			name:         "Simple Name",
			resource:     "my-network",
			expectedName: "my-network",
			expectedURL:  "",
		},
		{
			name:         "URL with Trailing Slash",
			resource:     "projects/my-project/global/networks/my-network/",
			expectedName: "",
			expectedURL:  "projects/my-project/global/networks/my-network/",
		},
		{
			name:         "Empty String",
			resource:     "",
			expectedName: "",
			expectedURL:  "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			name, url := parseResourceURL(tc.resource)
			assert.Equal(t, tc.expectedName, name)
			assert.Equal(t, tc.expectedURL, url)
		})
	}
}
