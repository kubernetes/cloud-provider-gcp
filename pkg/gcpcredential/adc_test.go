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

package gcpcredential

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const (
	adcTestImage             = "us-docker.pkg.dev/my-project/my-repo/my-image:latest"
	adcTestUserToken         = "user-access-token"
	adcTestImpersonatedToken = "impersonated-access-token"
)

// newADCTestServer returns a server that implements the OAuth2 token endpoint ("/token")
// and the IAM Credentials generateAccessToken endpoint ("/impersonate").
func newADCTestServer(t *testing.T, tokenStatus int) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		if tokenStatus != http.StatusOK {
			w.WriteHeader(tokenStatus)
			return
		}
		if err := r.ParseForm(); err != nil {
			t.Errorf("failed to parse token request: %v", err)
		}
		if got := r.Form.Get("grant_type"); got != "refresh_token" {
			t.Errorf("unexpected grant_type %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"access_token": %q, "token_type": "Bearer", "expires_in": 3600}`, adcTestUserToken)
	})
	mux.HandleFunc("/impersonate", func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.Header.Get("Authorization"), "Bearer "+adcTestUserToken; got != want {
			t.Errorf("impersonation request has Authorization header %q, want %q", got, want)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"accessToken": %q, "expireTime": %q}`, adcTestImpersonatedToken, time.Now().Add(time.Hour).Format(time.RFC3339))
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

func writeCredentialsFile(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "credentials.json")
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatalf("failed to write credentials file: %v", err)
	}
	return path
}

func TestApplicationDefaultCredentialsProvider(t *testing.T) {
	userCredentials := func(tokenURL string) string {
		return fmt.Sprintf(`{"type": "authorized_user", "client_id": "id", "client_secret": "secret", "refresh_token": "refresh", "token_uri": %q}`, tokenURL)
	}
	tests := []struct {
		name          string
		credentials   func(serverURL string) string
		tokenStatus   int
		expectedToken string
	}{
		{
			name:          "user credentials",
			credentials:   func(serverURL string) string { return userCredentials(serverURL + "/token") },
			tokenStatus:   http.StatusOK,
			expectedToken: adcTestUserToken,
		},
		{
			name: "service account impersonation",
			credentials: func(serverURL string) string {
				return fmt.Sprintf(`{"type": "impersonated_service_account", "service_account_impersonation_url": %q, "source_credentials": %s}`,
					serverURL+"/impersonate", userCredentials(serverURL+"/token"))
			},
			tokenStatus:   http.StatusOK,
			expectedToken: adcTestImpersonatedToken,
		},
		{
			name:        "token endpoint failure",
			credentials: func(serverURL string) string { return userCredentials(serverURL + "/token") },
			tokenStatus: http.StatusUnauthorized,
		},
		{
			name:        "invalid credentials file",
			credentials: func(string) string { return `{"type": "unknown"}` },
			tokenStatus: http.StatusOK,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := newADCTestServer(t, tc.tokenStatus)
			t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", writeCredentialsFile(t, tc.credentials(server.URL)))

			provider := &ApplicationDefaultCredentialsProvider{
				Client:               server.Client(),
				UseRegistryFromImage: true,
			}
			if !provider.Enabled() {
				t.Fatalf("expected provider to be enabled")
			}
			cfg := provider.Provide(adcTestImage)

			if tc.expectedToken == "" {
				if len(cfg) != 0 {
					t.Errorf("expected empty config, got %d entries", len(cfg))
				}
				return
			}
			for _, registry := range append([]string{"us-docker.pkg.dev"}, containerRegistryUrls...) {
				entry, ok := cfg[registry]
				if !ok {
					t.Errorf("missing config entry for %q", registry)
					continue
				}
				if entry.Username != "_token" {
					t.Errorf("unexpected username %q for %q", entry.Username, registry)
				}
				if entry.Password != tc.expectedToken {
					t.Errorf("unexpected password %q for %q, want %q", entry.Password, registry, tc.expectedToken)
				}
			}
		})
	}
}

func TestApplicationDefaultCredentialsProviderMissingFile(t *testing.T) {
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", filepath.Join(t.TempDir(), "does-not-exist.json"))
	provider := &ApplicationDefaultCredentialsProvider{}
	if cfg := provider.Provide(adcTestImage); len(cfg) != 0 {
		t.Errorf("expected empty config, got %d entries", len(cfg))
	}
}
