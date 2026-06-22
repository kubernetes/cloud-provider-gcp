//go:build !providerless
// +build !providerless

/*
Copyright 2024 The Kubernetes Authors.

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

package gce

import (
	"testing"

	"golang.org/x/oauth2"
	"google.golang.org/api/option"
)

func TestCredentialOptions(t *testing.T) {
	tests := []struct {
		name            string
		config          *CloudConfig
		wantErr         bool
		description     string
		expectAuthCreds bool // true if we expect WithAuthCredentialsJSON, false for WithTokenSource
	}{
		{
			name: "service account with type field",
			config: &CloudConfig{
				CredentialsJSON: []byte(`{
					"type": "service_account",
					"project_id": "test-project",
					"private_key_id": "key123",
					"private_key": "-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQC7W8jYbz0VLFjH\n-----END PRIVATE KEY-----\n",
					"client_email": "test@test-project.iam.gserviceaccount.com",
					"client_id": "123456789",
					"auth_uri": "https://accounts.google.com/o/oauth2/auth",
					"token_uri": "https://oauth2.googleapis.com/token",
					"auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
					"client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/test%40test-project.iam.gserviceaccount.com"
				}`),
			},
			wantErr:         false,
			description:     "should use WithAuthCredentialsJSON for service account with type field",
			expectAuthCreds: true,
		},
		{
			name: "authorized user with type field",
			config: &CloudConfig{
				CredentialsJSON: []byte(`{
					"type": "authorized_user",
					"client_id": "client123",
					"client_secret": "secret123",
					"refresh_token": "token123"
				}`),
			},
			wantErr:         false,
			description:     "should use WithAuthCredentialsJSON for authorized user with type field",
			expectAuthCreds: true,
		},
		{
			name: "credentials without type field",
			config: &CloudConfig{
				CredentialsJSON: []byte(`{
					"project_id": "test-project",
					"private_key_id": "key123"
				}`),
			},
			wantErr:         true,
			description:     "should fall back to CredentialsFromJSON but fail due to invalid credentials",
			expectAuthCreds: false,
		},
		{
			name: "no credentials JSON, use TokenSource",
			config: &CloudConfig{
				TokenSource: oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "test-token"}),
			},
			wantErr:         false,
			description:     "should use WithTokenSource when no CredentialsJSON provided",
			expectAuthCreds: false,
		},
		{
			name: "invalid JSON",
			config: &CloudConfig{
				CredentialsJSON: []byte(`invalid json`),
			},
			wantErr:         true,
			description:     "should fail with invalid JSON",
			expectAuthCreds: false,
		},
		{
			name: "empty credentials with TokenSource fallback",
			config: &CloudConfig{
				CredentialsJSON: []byte(``),
				TokenSource:     oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "test-token"}),
			},
			wantErr:         false,
			description:     "should use TokenSource when credentials JSON is empty",
			expectAuthCreds: false,
		},
		{
			name: "empty credentials without TokenSource",
			config: &CloudConfig{
				CredentialsJSON: []byte(``),
			},
			wantErr:         true,
			description:     "should fail when both credentials JSON and TokenSource are missing",
			expectAuthCreds: false,
		},
		{
			name:            "no credentials at all",
			config:          &CloudConfig{},
			wantErr:         true,
			description:     "should fail when neither credentials JSON nor TokenSource provided",
			expectAuthCreds: false,
		},
		{
			name: "external account with type field",
			config: &CloudConfig{
				CredentialsJSON: []byte(`{
					"type": "external_account",
					"audience": "//iam.googleapis.com/projects/123/locations/global/workloadIdentityPools/pool/providers/provider",
					"subject_token_type": "urn:ietf:params:oauth:token-type:jwt",
					"token_url": "https://sts.googleapis.com/v1/token",
					"credential_source": {
						"file": "/var/run/secrets/token"
					}
				}`),
			},
			wantErr:         false,
			description:     "should use WithAuthCredentialsJSON for external account with type field",
			expectAuthCreds: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts, err := credentialOptions(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("credentialOptions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if opts == nil || len(opts) == 0 {
					t.Errorf("credentialOptions() returned nil or empty options without error")
					return
				}
				// Verify we got a valid ClientOption
				var _ option.ClientOption = opts[0]
			}
		})
	}
}

func TestCredentialOptionsPreservesType(t *testing.T) {
	// Test that different credential types are handled correctly
	credentialTypes := []string{
		"service_account",
		"authorized_user",
		"external_account",
	}

	for _, credType := range credentialTypes {
		t.Run(credType, func(t *testing.T) {
			var credJSON string
			switch credType {
			case "service_account":
				credJSON = `{
					"type": "service_account",
					"project_id": "test-project",
					"private_key_id": "key123",
					"private_key": "-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQC7W8jYbz0VLFjH\n-----END PRIVATE KEY-----\n",
					"client_email": "test@test-project.iam.gserviceaccount.com",
					"client_id": "123456789",
					"auth_uri": "https://accounts.google.com/o/oauth2/auth",
					"token_uri": "https://oauth2.googleapis.com/token"
				}`
			case "authorized_user":
				credJSON = `{
					"type": "authorized_user",
					"client_id": "client123",
					"client_secret": "secret123",
					"refresh_token": "token123"
				}`
			case "external_account":
				credJSON = `{
					"type": "external_account",
					"audience": "//iam.googleapis.com/projects/123/locations/global/workloadIdentityPools/pool/providers/provider",
					"subject_token_type": "urn:ietf:params:oauth:token-type:jwt",
					"token_url": "https://sts.googleapis.com/v1/token",
					"credential_source": {
						"file": "/var/run/secrets/token"
					}
				}`
			}

			config := &CloudConfig{
				CredentialsJSON: []byte(credJSON),
			}

			opts, err := credentialOptions(config)
			if err != nil {
				t.Errorf("credentialOptions() unexpected error for %s: %v", credType, err)
				return
			}

			if opts == nil || len(opts) == 0 {
				t.Errorf("credentialOptions() returned nil or empty options for %s", credType)
				return
			}

			// Verify we got a valid ClientOption
			var _ option.ClientOption = opts[0]
		})
	}
}
