package gcpcredential

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type mockTransport struct {
	roundTripFunc func(req *http.Request) (*http.Response, error)
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.roundTripFunc(req)
}

func TestProvide_WorkloadIdentity(t *testing.T) {
	validToken := "dummyHeader.eyJpc3MiOiAiaHR0cHM6Ly9jb250YWluZXIuZ29vZ2xlYXBpcy5jb20vdjEvcHJvamVjdHMvbXktcHJvamVjdC9sb2NhdGlvbnMvdXMtY2VudHJhbDEvY2x1c3RlcnMvbXktY2x1c3RlciJ9.dummySignature"
	tests := []struct {
		name                      string
		identityProvider          string
		serviceAccountToken       string
		serviceAccountAnnotations map[string]string
		metadataResponses         map[string]string
		stsResponse               string
		wantAudience              string
		expectedToken             string
		projectID                 string
	}{
		{
			name:                "Direct Access Mode (Success)",
			identityProvider:    "https://container.googleapis.com/v1/projects/my-project/locations/us-central1/clusters/my-cluster",
			serviceAccountToken: validToken,
			serviceAccountAnnotations: map[string]string{
				"iam.gke.io/enable-wi-image-pull": "true",
			},
			projectID:     "my-project",
			stsResponse:   `{"access_token": "federated-token-xyz", "expires_in": 3600, "token_type": "Bearer"}`,
			wantAudience:  "identitynamespace:my-project.svc.id.goog:https://container.googleapis.com/v1/projects/my-project/locations/us-central1/clusters/my-cluster",
			expectedToken: "federated-token-xyz",
		},
		{
			name:                "Fallback to Node SA (Unannotated SA, bypass STS)",
			serviceAccountToken: validToken,
			metadataResponses: map[string]string{
				"project/project-id":                      "my-project",
				"instance/service-accounts/default/token": `{"access_token": "node-sa-token", "expires_in": 3600}`,
				"instance/service-accounts/default/email": "node-sa@project.gserviceaccount.com",
				"instance/service-accounts/":              "default/\n",
			},
			stsResponse:   `{"access_token": "should-not-be-called", "expires_in": 3600, "token_type": "Bearer"}`,
			expectedToken: "node-sa-token",
		},
		{
			name:                "Fail Fast - Annotated SA for WIF, STS Fails",
			identityProvider:    "https://container.googleapis.com/v1/projects/my-project/locations/us-central1/clusters/my-cluster",
			serviceAccountToken: validToken,
			serviceAccountAnnotations: map[string]string{
				"iam.gke.io/enable-wi-image-pull": "true",
			},
			projectID: "my-project",
			metadataResponses: map[string]string{
				"instance/service-accounts/default/token": `{"access_token": "node-sa-token", "expires_in": 3600}`,
			},
			stsResponse:   "error",
			wantAudience:  "identitynamespace:my-project.svc.id.goog:https://container.googleapis.com/v1/projects/my-project/locations/us-central1/clusters/my-cluster",
			expectedToken: "",
		},
		{
			name:                "Fail Fast - Annotated SA for WIF, Token is Empty",
			identityProvider:    "https://container.googleapis.com/v1/projects/my-project/locations/us-central1/clusters/my-cluster",
			serviceAccountToken: "",
			serviceAccountAnnotations: map[string]string{
				"iam.gke.io/enable-wi-image-pull": "true",
			},
			projectID: "my-project",
			metadataResponses: map[string]string{
				"instance/service-accounts/default/token": `{"access_token": "node-sa-token", "expires_in": 3600}`,
			},
			stsResponse:   `{"access_token": "should-not-be-called", "expires_in": 3600, "token_type": "Bearer"}`,
			expectedToken: "",
		},
		{
			name:                "Fallback to Node SA - Annotated but Feature Disabled",
			identityProvider:    "",
			serviceAccountToken: validToken,
			serviceAccountAnnotations: map[string]string{
				"iam.gke.io/enable-wi-image-pull": "true",
			},
			metadataResponses: map[string]string{
				"project/project-id":                      "my-project",
				"instance/service-accounts/default/token": `{"access_token": "node-sa-token", "expires_in": 3600}`,
				"instance/service-accounts/default/email": "node-sa@project.gserviceaccount.com",
				"instance/service-accounts/":              "default/\n",
			},
			stsResponse:   `{"access_token": "should-not-be-called", "expires_in": 3600, "token_type": "Bearer"}`,
			expectedToken: "node-sa-token",
		},
		{
			name:                "Direct Access Mode - Configured Identity Provider (Success)",
			identityProvider:    "https://custom-provider.com",
			serviceAccountToken: validToken,
			serviceAccountAnnotations: map[string]string{
				"iam.gke.io/enable-wi-image-pull": "true",
			},
			projectID:     "my-project",
			stsResponse:   `{"access_token": "federated-token-custom", "expires_in": 3600, "token_type": "Bearer"}`,
			wantAudience:  "identitynamespace:my-project.svc.id.goog:https://custom-provider.com",
			expectedToken: "federated-token-custom",
		},
		{
			name:                "Direct Access Mode - With Pre-configured Project ID (Success)",
			identityProvider:    "https://container.googleapis.com/v1/projects/my-project/locations/us-central1/clusters/my-cluster",
			serviceAccountToken: validToken,
			serviceAccountAnnotations: map[string]string{
				"iam.gke.io/enable-wi-image-pull": "true",
			},
			projectID:     "pre-configured-project",
			stsResponse:   `{"access_token": "federated-token-pre", "expires_in": 3600, "token_type": "Bearer"}`,
			wantAudience:  "identitynamespace:pre-configured-project.svc.id.goog:https://container.googleapis.com/v1/projects/my-project/locations/us-central1/clusters/my-cluster",
			expectedToken: "federated-token-pre",
		},
		{
			name:                "Fail Fast - Project ID is Empty",
			identityProvider:    "https://container.googleapis.com/v1/projects/my-project/locations/us-central1/clusters/my-cluster",
			serviceAccountToken: validToken,
			serviceAccountAnnotations: map[string]string{
				"iam.gke.io/enable-wi-image-pull": "true",
			},
			projectID:     "",
			expectedToken: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mTransport := &mockTransport{
				roundTripFunc: func(req *http.Request) (*http.Response, error) {
					// Handle Metadata Server requests
					if req.URL.Host == "metadata.google.internal." {
						path := strings.TrimPrefix(req.URL.Path, "/computeMetadata/v1/")
						if resp, ok := tc.metadataResponses[path]; ok {
							return &http.Response{
								StatusCode: http.StatusOK,
								Body:       io.NopCloser(bytes.NewBufferString(resp)),
								Header:     make(http.Header),
							}, nil
						}
						return &http.Response{
							StatusCode: http.StatusNotFound,
							Body:       io.NopCloser(bytes.NewBufferString("")),
						}, nil
					}

					// Handle STS Requests
					if req.URL.Host == "sts.googleapis.com" && req.URL.Path == "/v1/token" {
						if tc.stsResponse == "error" {
							return &http.Response{
								StatusCode: http.StatusBadRequest,
								Body:       io.NopCloser(bytes.NewBufferString(`{"error": "invalid_grant"}`)),
							}, nil
						}

						// Verify audience if specified
						if tc.wantAudience != "" {
							bodyBytes, err := io.ReadAll(req.Body)
							if err != nil {
								t.Errorf("failed to read STS request body: %v", err)
							}
							var stsReq stsTokenExchangeRequest
							if err := json.Unmarshal(bodyBytes, &stsReq); err != nil {
								t.Errorf("failed to unmarshal STS request body: %v", err)
							}
							if stsReq.Audience != tc.wantAudience {
								t.Errorf("unexpected STS audience: got=%q, want=%q", stsReq.Audience, tc.wantAudience)
							}
						}

						return &http.Response{
							StatusCode: http.StatusOK,
							Body:       io.NopCloser(bytes.NewBufferString(tc.stsResponse)),
						}, nil
					}

					return &http.Response{
						StatusCode: http.StatusNotFound,
						Body:       io.NopCloser(bytes.NewBufferString("")),
					}, nil
				},
			}

			httpClient := &http.Client{
				Transport: mTransport,
			}

			provider := &ContainerRegistryProvider{
				MetadataProvider: MetadataProvider{
					Client: httpClient,
				},
				UseRegistryFromImage: true,
				K8sType:              K8sTypeGKE,
			}
			provider.KSAToken = tc.serviceAccountToken
			provider.ServiceAccountAnnotations = tc.serviceAccountAnnotations
			provider.WIConfig.IdentityProvider = tc.identityProvider
			provider.WIConfig.ProjectID = tc.projectID

			cfg := provider.Provide("us-central1-docker.pkg.dev/my-project/my-repo/my-image:latest")

			if tc.expectedToken == "" {
				for _, entry := range cfg {
					if entry.Password != "" {
						t.Errorf("expected no token, got: %s", entry.Password)
					}
				}
			} else {
				found := false
				for _, entry := range cfg {
					if entry.Password == tc.expectedToken {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected token %q not found in config: %+v", tc.expectedToken, cfg)
				}
			}
		})
	}
}

func TestProvide_SelfManagedWorkloadIdentity(t *testing.T) {
	validToken := "dummyHeader.eyJpc3MiOiAic2VsZi1tYW5hZ2VkLWlzc3VlciJ9.dummySignature"
	const federatedToken = "self-managed-federated-token"
	const image = "us-central1-docker.pkg.dev/my-project/my-repo/my-image:latest"

	tests := []struct {
		name          string
		wiConfig      WIConfig
		wantAudience  string
		wantToken     string
		wantSTSCalled bool
	}{
		{
			name: "success",
			wiConfig: WIConfig{
				ProjectNumber: "123456789",
				PoolID:        "pool-id",
				ProviderID:    "provider-id",
			},
			wantAudience:  "//iam.googleapis.com/projects/123456789/locations/global/workloadIdentityPools/pool-id/providers/provider-id",
			wantToken:     federatedToken,
			wantSTSCalled: true,
		},
		{
			name: "missing provider id fails before STS",
			wiConfig: WIConfig{
				ProjectNumber: "123456789",
				PoolID:        "pool-id",
			},
			wantSTSCalled: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stsCalled := false
			httpClient := &http.Client{
				Transport: &mockTransport{
					roundTripFunc: func(req *http.Request) (*http.Response, error) {
						if req.URL.Host != "sts.googleapis.com" || req.URL.Path != "/v1/token" {
							t.Fatalf("unexpected request to %s", req.URL.String())
						}

						stsCalled = true
						var stsReq stsTokenExchangeRequest
						if err := json.NewDecoder(req.Body).Decode(&stsReq); err != nil {
							t.Fatalf("failed to decode STS request: %v", err)
						}
						if stsReq.Audience != tc.wantAudience {
							t.Fatalf("unexpected STS audience: got=%q, want=%q", stsReq.Audience, tc.wantAudience)
						}
						if stsReq.SubjectToken != validToken {
							t.Fatalf("unexpected subject token: got=%q, want=%q", stsReq.SubjectToken, validToken)
						}

						return &http.Response{
							StatusCode: http.StatusOK,
							Body:       io.NopCloser(bytes.NewBufferString(`{"access_token": "` + federatedToken + `", "expires_in": 3600, "token_type": "Bearer"}`)),
							Header:     make(http.Header),
						}, nil
					},
				},
			}

			provider := &ContainerRegistryProvider{
				MetadataProvider: MetadataProvider{
					Client: httpClient,
				},
				UseRegistryFromImage: true,
				K8sType:              K8sTypeSelfManaged,
				KSAToken:             validToken,
				WIConfig:             tc.wiConfig,
			}

			cfg := provider.Provide(image)
			if stsCalled != tc.wantSTSCalled {
				t.Fatalf("STS call mismatch: got=%t, want=%t", stsCalled, tc.wantSTSCalled)
			}
			if tc.wantToken == "" {
				if len(cfg) != 0 {
					t.Fatalf("expected empty config, got: %+v", cfg)
				}
				return
			}
			for _, entry := range cfg {
				if entry.Password == tc.wantToken {
					return
				}
			}
			t.Fatalf("expected token %q not found in config: %+v", tc.wantToken, cfg)
		})
	}
}
