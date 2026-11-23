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
		metadataStatusCodes       map[string]int
		stsResponse               string
		wantAudience              string
		wantSTSHost               string
		expectedToken             string
		projectID                 string
		workloadPool              string
		stsEndpoint               string
		image                     string
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
			wantSTSHost:   "sts.googleapis.com",
			expectedToken: "federated-token-xyz",
		},
		{
			name:                "Direct Access Mode - Non-Default Universe Workload Pool and Universe Domain Metadata (Success)",
			identityProvider:    "https://container.example-universe.goog/v1/projects/example-universe:my-project/locations/us-central1/clusters/my-cluster",
			serviceAccountToken: validToken,
			serviceAccountAnnotations: map[string]string{
				"iam.gke.io/enable-wi-image-pull": "true",
			},
			projectID:    "my-project",
			workloadPool: "my-project.example-universe.svc.id.goog",
			metadataResponses: map[string]string{
				"universe/universe-domain": "example-universe.goog\n",
			},
			stsResponse:   `{"access_token": "federated-token-universe", "expires_in": 3600, "token_type": "Bearer"}`,
			wantAudience:  "identitynamespace:my-project.example-universe.svc.id.goog:https://container.example-universe.goog/v1/projects/example-universe:my-project/locations/us-central1/clusters/my-cluster",
			wantSTSHost:   "sts.example-universe.goog",
			expectedToken: "federated-token-universe",
		},
		{
			name:                "Direct Access Mode - Explicit STS Endpoint Override (Success)",
			identityProvider:    "https://container.example-universe.goog/v1/projects/example-universe:my-project/locations/us-central1/clusters/my-cluster",
			serviceAccountToken: validToken,
			serviceAccountAnnotations: map[string]string{
				"iam.gke.io/enable-wi-image-pull": "true",
			},
			workloadPool:  "my-project.example-universe.svc.id.goog",
			stsEndpoint:   "https://sts.custom-universe.goog/v1/token",
			stsResponse:   `{"access_token": "federated-token-override", "expires_in": 3600, "token_type": "Bearer"}`,
			wantAudience:  "identitynamespace:my-project.example-universe.svc.id.goog:https://container.example-universe.goog/v1/projects/example-universe:my-project/locations/us-central1/clusters/my-cluster",
			wantSTSHost:   "sts.custom-universe.goog",
			expectedToken: "federated-token-override",
			image:         "docker.pkg.example-universe.goog/example-universe:my-project/my-repo/my-image:latest",
		},
		{
			name:                "Direct Access Mode - Base STS Endpoint Without Path Defaults to /v1/token (Success)",
			identityProvider:    "https://container.example-universe.goog/v1/projects/example-universe:my-project/locations/us-central1/clusters/my-cluster",
			serviceAccountToken: validToken,
			serviceAccountAnnotations: map[string]string{
				"iam.gke.io/enable-wi-image-pull": "true",
			},
			workloadPool:  "my-project.example-universe.svc.id.goog",
			stsEndpoint:   "https://sts.example-universe.goog",
			stsResponse:   `{"access_token": "federated-token-base-url", "expires_in": 3600, "token_type": "Bearer"}`,
			wantAudience:  "identitynamespace:my-project.example-universe.svc.id.goog:https://container.example-universe.goog/v1/projects/example-universe:my-project/locations/us-central1/clusters/my-cluster",
			wantSTSHost:   "sts.example-universe.goog",
			expectedToken: "federated-token-base-url",
			image:         "docker.pkg.example-universe.goog/example-universe:my-project/my-repo/my-image:latest",
		},
		{
			name:                "Fail Fast - Non-404 Universe Domain Metadata Error Does Not Fallback to Default STS",
			identityProvider:    "https://container.example-universe.goog/v1/projects/example-universe:my-project/locations/us-central1/clusters/my-cluster",
			serviceAccountToken: validToken,
			serviceAccountAnnotations: map[string]string{
				"iam.gke.io/enable-wi-image-pull": "true",
			},
			workloadPool: "my-project.example-universe.svc.id.goog",
			metadataStatusCodes: map[string]int{
				"universe/universe-domain": http.StatusServiceUnavailable,
			},
			metadataResponses: map[string]string{
				"instance/service-accounts/default/token": `{"access_token": "node-sa-token", "expires_in": 3600}`,
			},
			stsResponse:   `{"access_token": "should-not-hit-default-sts", "expires_in": 3600, "token_type": "Bearer"}`,
			wantSTSHost:   "sts.googleapis.com",
			expectedToken: "",
			image:         "docker.pkg.example-universe.goog/example-universe:my-project/my-repo/my-image:latest",
		},
		{
			name:                "Fail Fast - Non-HTTPS STS Endpoint Override Rejected",
			identityProvider:    "https://container.example-universe.goog/v1/projects/example-universe:my-project/locations/us-central1/clusters/my-cluster",
			serviceAccountToken: validToken,
			serviceAccountAnnotations: map[string]string{
				"iam.gke.io/enable-wi-image-pull": "true",
			},
			workloadPool:  "my-project.example-universe.svc.id.goog",
			stsEndpoint:   "http://sts.example-universe.goog/v1/token",
			stsResponse:   `{"access_token": "should-not-be-called", "expires_in": 3600, "token_type": "Bearer"}`,
			wantSTSHost:   "sts.example-universe.goog",
			expectedToken: "",
		},
		{
			name:                "Direct Access Mode - Default Universe Domain Metadata (Success)",
			identityProvider:    "https://container.googleapis.com/v1/projects/my-project/locations/us-central1/clusters/my-cluster",
			serviceAccountToken: validToken,
			serviceAccountAnnotations: map[string]string{
				"iam.gke.io/enable-wi-image-pull": "true",
			},
			workloadPool: "my-project.svc.id.goog",
			metadataResponses: map[string]string{
				"universe/universe-domain": "googleapis.com",
			},
			stsResponse:   `{"access_token": "federated-token-default-universe", "expires_in": 3600, "token_type": "Bearer"}`,
			wantAudience:  "identitynamespace:my-project.svc.id.goog:https://container.googleapis.com/v1/projects/my-project/locations/us-central1/clusters/my-cluster",
			wantSTSHost:   "sts.googleapis.com",
			expectedToken: "federated-token-default-universe",
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
						if statusCode, ok := tc.metadataStatusCodes[path]; ok {
							return &http.Response{
								StatusCode: statusCode,
								Body:       io.NopCloser(bytes.NewBufferString("metadata error")),
								Header:     make(http.Header),
							}, nil
						}
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
					expectedSTSHost := tc.wantSTSHost
					if expectedSTSHost == "" {
						expectedSTSHost = "sts.googleapis.com"
					}
					if req.URL.Host == expectedSTSHost && req.URL.Path == "/v1/token" {
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
			}
			provider.KSAToken = tc.serviceAccountToken
			provider.ServiceAccountAnnotations = tc.serviceAccountAnnotations
			provider.IdentityProvider = tc.identityProvider
			provider.ProjectID = tc.projectID
			provider.WorkloadPool = tc.workloadPool
			provider.STSEndpoint = tc.stsEndpoint

			image := tc.image
			if image == "" {
				image = "us-central1-docker.pkg.dev/my-project/my-repo/my-image:latest"
			}
			cfg := provider.Provide(image)

			if tc.expectedToken == "" {
				for _, entry := range cfg {
					if entry.Password != "" {
						t.Errorf("expected no token, got: %s", entry.Password)
					}
				}
			} else {
				registry, _, _ := strings.Cut(image, "/")
				if entry, ok := cfg[registry]; !ok || entry.Password != tc.expectedToken {
					t.Errorf("expected token %q for registry %q in config: %+v", tc.expectedToken, registry, cfg)
				}
			}
		})
	}
}
