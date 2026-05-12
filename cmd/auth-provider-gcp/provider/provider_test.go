/*
Copyright 2020 The Kubernetes Authors.

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
package provider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/cloud-provider-gcp/pkg/gcpcredential"
	credentialproviderapi "k8s.io/kubelet/pkg/apis/credentialprovider/v1"
)

const (
	dummyToken       = "ya26.lots-of-indiscernible-garbage"
	email            = "1234@project.gserviceaccount.com"
	expectedUsername = "_token"
	expectedCacheKey = credentialproviderapi.ImagePluginCacheKeyType
	dummyImage       = "registry.k8s.io/pause"
)

func hasURL(url string, response *credentialproviderapi.CredentialProviderResponse) bool {
	_, ok := response.Auth[url]
	return ok
}

func TestContainerRegistry(t *testing.T) {
	registryURL := strings.Split(dummyImage, "/")[0]
	// Taken from from pkg/credentialprovider/gcp/metadata_test.go in kubernetes/kubernetes
	gcpRegistryURL := "container.cloud.google.com"
	token := &gcpcredential.TokenBlob{AccessToken: dummyToken} // Fake value for testing.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defaultPrefix := "/computeMetadata/v1/instance/service-accounts/default/"
		// Only serve the URL key and the value endpoint
		switch r.URL.Path {
		case defaultPrefix + "scopes":
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `["%s.read_write"]`, gcpcredential.StorageScopePrefix)
		case defaultPrefix + "email":
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, email)
		case defaultPrefix + "token":
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			bytes, err := json.Marshal(token)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			fmt.Fprintln(w, string(bytes))
		case "/computeMetadata/v1/instance/service-accounts/":
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, "default/\ncustom")
		default:
			http.Error(w, "", http.StatusNotFound)
		}
	}))
	defer server.Close()
	// Make a transport that reroutes all traffic to the example server
	transport := utilnet.SetTransportDefaults(&http.Transport{
		Proxy: func(req *http.Request) (*url.URL, error) {
			return url.Parse(server.URL + req.URL.Path)
		},
	})
	provider := MakeRegistryProvider(transport)
	response, err := GetResponse(credentialproviderapi.CredentialProviderRequest{Image: dummyImage}, provider)
	if err != nil {
		t.Fatalf("Unexpected error while getting response: %s", err.Error())
	}

	if !hasURL(registryURL, response) || !hasURL(gcpRegistryURL, response) {
		if !hasURL(registryURL, response) {
			t.Errorf("URL %s expected in response, not found (response: %s)", registryURL, response.Auth)
		}
		if !hasURL(gcpRegistryURL, response) {
			t.Errorf("URL %s expected in response, not found (response: %s)", gcpRegistryURL, response.Auth)
		}
	}
	if expectedCacheKey != response.CacheKeyType {
		t.Errorf("Expected %s as cache key (found %s instead)", expectedCacheKey, response.CacheKeyType)
	}
	for _, auth := range response.Auth {
		if expectedUsername != auth.Username {
			t.Errorf("Expected username %s not found (username: %s)", expectedUsername, auth.Username)
		}
		if dummyToken != auth.Password {
			t.Errorf("Expected password %s not found (password: %s)", dummyToken, auth.Password)
		}
	}
}

func TestConfigProvider(t *testing.T) {
	// Taken from from pkg/credentialprovider/gcp/metadata_test.go in kubernetes/kubernetes
	registryURL := "hello.kubernetes.io"
	email := "foo@bar.baz"
	username := "foo"
	password := "bar" // Fake value for testing.
	auth := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", username, password)))
	sampleDockerConfig := fmt.Sprintf(`{
   "https://%s": {
     "email": %q,
     "auth": %q
   }
}`, registryURL, email, auth)

	const probeEndpoint = "/computeMetadata/v1/"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only serve the one metadata key.
		if probeEndpoint == r.URL.Path {
			w.WriteHeader(http.StatusOK)
		} else if strings.HasSuffix(gcpcredential.DockerConfigKey, r.URL.Path) {
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintln(w, sampleDockerConfig)
		} else {
			http.Error(w, "", http.StatusNotFound)
		}
	}))
	defer server.Close()

	// Make a transport that reroutes all traffic to the example server
	transport := utilnet.SetTransportDefaults(&http.Transport{
		Proxy: func(req *http.Request) (*url.URL, error) {
			return url.Parse(server.URL + req.URL.Path)
		},
	})
	provider := MakeDockerConfigProvider(transport)
	response, err := GetResponse(credentialproviderapi.CredentialProviderRequest{Image: dummyImage}, provider)
	if err != nil {
		t.Fatalf("Unexpected error while getting response: %s", err.Error())
	}
	if expectedCacheKey != response.CacheKeyType {
		t.Errorf("Expected %s as cache key (found %s instead)", expectedCacheKey, response.CacheKeyType)
	}
	for _, auth := range response.Auth {
		if username != auth.Username {
			t.Errorf("Expected username %s not found (username: %s)", username, auth.Username)
		}
		if password != auth.Password {
			t.Errorf("Expected password %s not found (password: %s)", password, auth.Password)
		}
	}
}

func TestConfigURLProvider(t *testing.T) {
	// Taken from from pkg/credentialprovider/gcp/metadata_test.go in kubernetes/kubernetes
	registryURL := "hello.kubernetes.io"
	email := "foo@bar.baz"
	username := "foo"
	password := "bar" // Fake value for testing.
	auth := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", username, password)))
	sampleDockerConfig := fmt.Sprintf(`{
   "https://%s": {
     "email": %q,
     "auth": %q
   }
}`, registryURL, email, auth)
	const probeEndpoint = "/computeMetadata/v1/"
	const valueEndpoint = "/my/value"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only serve the URL key and the value endpoint
		if probeEndpoint == r.URL.Path {
			w.WriteHeader(http.StatusOK)
		} else if valueEndpoint == r.URL.Path {
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintln(w, sampleDockerConfig)
		} else if strings.HasSuffix(gcpcredential.DockerConfigURLKey, r.URL.Path) {
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/text")
			fmt.Fprint(w, "http://foo.bar.com"+valueEndpoint)
		} else {
			http.Error(w, "", http.StatusNotFound)
		}
	}))
	defer server.Close()
	// Make a transport that reroutes all traffic to the example server
	transport := utilnet.SetTransportDefaults(&http.Transport{
		Proxy: func(req *http.Request) (*url.URL, error) {
			return url.Parse(server.URL + req.URL.Path)
		},
	})

	provider := MakeDockerConfigURLProvider(transport)
	response, err := GetResponse(credentialproviderapi.CredentialProviderRequest{Image: dummyImage}, provider)
	if err != nil {
		t.Fatalf("Unexpected error while getting response: %s", err.Error())
	}
	if expectedCacheKey != response.CacheKeyType {
		t.Errorf("Expected %s as cache key (found %s instead)", expectedCacheKey, response.CacheKeyType)
	}
	for _, auth := range response.Auth {
		if username != auth.Username {
			t.Errorf("Expected username %s not found (username: %s)", username, auth.Username)
		}
		if password != auth.Password {
			t.Errorf("Expected password %s not found (password: %s)", password, auth.Password)
		}
	}
}

func TestK8sSAWIFProvider(t *testing.T) {
	registryURL := strings.Split(dummyImage, "/")[0]
	gcpRegistryURL := "container.cloud.google.com"

	projectNum := "123456789"
	poolId := "test-pool"
	providerId := "test-provider"

	os.Setenv("GCP_WIF_PROJECT_NUMBER", projectNum)
	os.Setenv("GCP_WIF_POOL_ID", poolId)
	os.Setenv("GCP_WIF_PROVIDER_ID", providerId)
	defer os.Unsetenv("GCP_WIF_PROJECT_NUMBER")
	defer os.Unsetenv("GCP_WIF_POOL_ID")
	defer os.Unsetenv("GCP_WIF_PROVIDER_ID")

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/v1/token") {
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			tokenResponse := map[string]interface{}{
				"access_token":      dummyToken,
				"expires_in":        3600,
				"issued_token_type": "urn:ietf:params:oauth:token-type:access_token",
			}
			resp, _ := json.Marshal(tokenResponse)
			if _, err := w.Write(resp); err != nil {
				t.Fatalf("write token response: %v", err)
			}
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}

	transport := server.Client().Transport.(*http.Transport).Clone()
	transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		if strings.HasPrefix(addr, "sts.googleapis.com:") {
			var d net.Dialer
			return d.DialContext(ctx, network, serverURL.Host)
		}
		var d net.Dialer
		return d.DialContext(ctx, network, addr)
	}
	transport.TLSClientConfig.ServerName = "127.0.0.1"

	provider, err := MakeK8sSAWIFProvider(transport)
	if err != nil {
		t.Fatalf("Unexpected error while creating provider: %v", err)
	}

	if provider == nil {
		t.Fatalf("Expected K8sSAWIFProvider but got nil")
	}

	if provider.WIFConfig.ProjectNumber != projectNum {
		t.Errorf("Expected project number %s, got %s", projectNum, provider.WIFConfig.ProjectNumber)
	}
	if provider.WIFConfig.PoolId != poolId {
		t.Errorf("Expected pool ID %s, got %s", poolId, provider.WIFConfig.PoolId)
	}
	if provider.WIFConfig.ProviderId != providerId {
		t.Errorf("Expected provider ID %s, got %s", providerId, provider.WIFConfig.ProviderId)
	}
	if !provider.UseRegistryFromImage {
		t.Errorf("Expected UseRegistryFromImage to be true")
	}
	if provider.StsService == nil {
		t.Errorf("Expected StsService to be configured")
	}

	response, err := GetResponse(credentialproviderapi.CredentialProviderRequest{Image: dummyImage}, provider)
	if err != nil {
		t.Fatalf("Unexpected error while getting response: %v", err)
	}

	if !hasURL(registryURL, response) || !hasURL(gcpRegistryURL, response) {
		if !hasURL(registryURL, response) {
			t.Errorf("URL %s expected in response, not found (response: %s)", registryURL, response.Auth)
		}
		if !hasURL(gcpRegistryURL, response) {
			t.Errorf("URL %s expected in response, not found (response: %s)", gcpRegistryURL, response.Auth)
		}
	}

	if apiKind != response.TypeMeta.Kind {
		t.Errorf("Expected Kind %s, got %s", apiKind, response.TypeMeta.Kind)
	}
	if apiVersion != response.TypeMeta.APIVersion {
		t.Errorf("Expected APIVersion %s, got %s", apiVersion, response.TypeMeta.APIVersion)
	}
	if expectedCacheKey != response.CacheKeyType {
		t.Errorf("Expected %s as cache key (found %s instead)", expectedCacheKey, response.CacheKeyType)
	}
	for _, auth := range response.Auth {
		if expectedUsername != auth.Username {
			t.Errorf("Expected username %s not found (username: %s)", expectedUsername, auth.Username)
		}
		if dummyToken != auth.Password {
			t.Errorf("Expected password %s not found (password: %s)", dummyToken, auth.Password)
		}
	}
}
