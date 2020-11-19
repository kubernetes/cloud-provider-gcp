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
package plugin

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/cloud-provider-gcp/cmd/auth-provider-gcp/gcpcredential"
	credentialproviderapi "k8s.io/kubelet/pkg/apis/credentialprovider"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

const (
	serviceAccountsEndpoint = "/computeMetadata/v1/instance/service-accounts/"
	defaultEndpoint         = "/computeMetadata/v1/instance/service-accounts/default/"
	scopeEndpoint           = defaultEndpoint + "scopes"
	emailEndpoint           = defaultEndpoint + "email"
	tokenEndpoint           = defaultEndpoint + "token"
	dummyToken              = "ya26.lots-of-indiscernible-garbage"
	email                   = "1234@project.gserviceaccount.com"
	expectedUsername        = "_token"
)

func hasURL(url string, response *credentialproviderapi.CredentialProviderResponse) bool {
	_, ok := response.Auth[url]
	return ok
}

func usernameMatches(expectedUsername string, auth credentialproviderapi.AuthConfig) bool {
	return auth.Username == expectedUsername
}

func passwordMatches(expectedPassword string, auth credentialproviderapi.AuthConfig) bool {
	return auth.Password == expectedPassword
}

func TestContainerRegistry(t *testing.T) {
	// Taken from from pkg/credentialprovider/gcp/metadata_test.go in k/k
	registryURL := "container.cloud.google.com"
	token := &gcpcredential.TokenBlob{AccessToken: dummyToken} // Fake value for testing.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only serve the URL key and the value endpoint
		if scopeEndpoint == r.URL.Path {
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `["%s.read_write"]`, gcpcredential.StorageScopePrefix)
		} else if emailEndpoint == r.URL.Path {
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, email)
		} else if tokenEndpoint == r.URL.Path {
			w.WriteHeader(http.StatusOK)
			w.Header().Set("Content-Type", "application/json")
			bytes, err := json.Marshal(token)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			fmt.Fprintln(w, string(bytes))
		} else if serviceAccountsEndpoint == r.URL.Path {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, "default/\ncustom")
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
	provider := MakeRegistryProvider(transport)
	response, err := GetResponse(provider)
	if err != nil {
		t.Fatalf("Unexpected error while getting response: %s", err.Error())
	}
	if hasURL(registryURL, response) == false {
		t.Errorf("URL %s expected in response, not found (response: %s)", registryURL, response.Auth)
	}
	for _, auth := range response.Auth {
		if usernameMatches(expectedUsername, auth) == false {
			t.Errorf("Expected username %s not found (username: %s)", expectedUsername, auth.Username)
		}
		if passwordMatches(dummyToken, auth) == false {
			t.Errorf("Expected password %s not found (password: %s)", dummyToken, auth.Password)
		}
	}
}

func TestConfigProvider(t *testing.T) {
	// Taken from from pkg/credentialprovider/gcp/metadata_test.go in k/k
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
	response, err := GetResponse(provider)
	if err != nil {
		t.Fatalf("Unexpected error while getting response: %s", err.Error())
	}
	for _, auth := range response.Auth {
		if usernameMatches(username, auth) == false {
			t.Errorf("Expected username %s not found (username: %s)", username, auth.Username)
		}
		if passwordMatches(password, auth) == false {
			t.Errorf("Expected password %s not found (password: %s)", password, auth.Password)
		}
	}
}

func TestConfigURLProvider(t *testing.T) {
	// Taken from from pkg/credentialprovider/gcp/metadata_test.go in k/k
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
	response, err := GetResponse(provider)
	if err != nil {
		t.Fatalf("Unexpected error while getting response: %s", err.Error())
	}
	for _, auth := range response.Auth {
		if usernameMatches(username, auth) == false {
			t.Errorf("Expected username %s not found (username: %s)", username, auth.Username)
		}
		if passwordMatches(password, auth) == false {
			t.Errorf("Expected password %s not found (password: %s)", password, auth.Password)
		}
	}
}
