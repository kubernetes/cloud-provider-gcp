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

package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

func TestAllowedPaths(t *testing.T) {
	for _, tc := range []struct {
		spec string
		want []string
	}{
		{"", []string{"/", "/computeMetadata/v1/instance/zone", "/computeMetadata/v1/project/numeric-project-id", "/computeMetadata/v1/project/project-id"}},
		{" +instance/machine-type, -project/numeric-project-id ,", []string{"/", "/computeMetadata/v1/instance/machine-type", "/computeMetadata/v1/instance/zone", "/computeMetadata/v1/project/project-id"}},
		{"-instance/zone,+instance/zone", []string{"/", "/computeMetadata/v1/instance/zone", "/computeMetadata/v1/project/numeric-project-id", "/computeMetadata/v1/project/project-id"}},
		{"-project/project-id,-project/numeric-project-id,-instance/zone", []string{"/"}},
	} {
		got, err := allowedPaths(tc.spec)
		if err != nil {
			t.Errorf("allowedPaths(%q): %v", tc.spec, err)
			continue
		}
		var paths []string
		for p := range got {
			paths = append(paths, p)
		}
		slices.Sort(paths)
		if !slices.Equal(paths, tc.want) {
			t.Errorf("allowedPaths(%q) = %q, want %q", tc.spec, paths, tc.want)
		}
	}
	for _, spec := range []string{
		"instance/zone",
		"+",
		"+instance/service-accounts/default/token",
		"+instance/attributes/kube-env",
		"+/computeMetadata/v1/instance/zone",
		"+instance/zone/",
		"+Instance/Zone",
	} {
		if _, err := allowedPaths(spec); err == nil {
			t.Errorf("allowedPaths(%q): expected error", spec)
		}
	}
}

type upstreamRequest struct {
	method, uri string
	headers     http.Header
}

// TestHandler runs the proxy in front of a fake metadata server and checks what crosses it in each
// direction.
func TestHandler(t *testing.T) {
	// The transport must not honour proxy environment variables.
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:1")
	t.Setenv("http_proxy", "http://127.0.0.1:1")

	var seen []upstreamRequest
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, upstreamRequest{r.Method, r.URL.RequestURI(), r.Header.Clone()})
		w.Header().Set("Metadata-Flavor", "Google")
		w.Header().Set("Content-Type", "application/text")
		w.Header().Set("Server", "Metadata Server for VM")
		w.Header().Set("ETag", "abc")
		switch r.URL.Path {
		case "/computeMetadata/v1/instance/machine-type":
			http.Redirect(w, r, "/computeMetadata/v1/instance/attributes/", http.StatusFound)
		case "/computeMetadata/v1/instance/id":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(make([]byte, maxResponseBytes+1))
		default:
			_, _ = io.WriteString(w, "ok:"+r.URL.Path)
		}
	}))
	defer upstream.Close()

	allowed, err := allowedPaths("+instance/machine-type,+instance/id")
	if err != nil {
		t.Fatal(err)
	}
	h, err := newHandler(upstream.URL, "http://[fd20:ce::254]", allowed)
	if err != nil {
		t.Fatal(err)
	}
	proxy := httptest.NewServer(h)
	defer proxy.Close()

	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	do := func(method, path string) (*http.Response, string) {
		t.Helper()
		req, err := http.NewRequest(method, proxy.URL+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Metadata-Flavor", "Google")
		req.Header.Set("X-Forwarded-For", "10.0.0.1")
		req.Header.Set("Authorization", "Bearer secret")
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp, strings.TrimSpace(string(b))
	}

	// The proxy fetches allowed paths exactly, drops the query string, forwards only
	// Metadata-Flavor, and returns only Content-Type and Metadata-Flavor.
	for _, tc := range []struct{ in, want string }{
		{"/", "/"},
		{"/computeMetadata/v1/project/project-id", "/computeMetadata/v1/project/project-id"},
		{"/computeMetadata/v1/instance/zone", "/computeMetadata/v1/instance/zone"},
		{"/computeMetadata/v1/instance/zone?alt=json&recursive=true", "/computeMetadata/v1/instance/zone"},
		{"/computeMetadata/v1/instance/zone?wait_for_change=true", "/computeMetadata/v1/instance/zone"},
	} {
		before := len(seen)
		resp, body := do(http.MethodGet, tc.in)
		if resp.StatusCode != http.StatusOK || body != "ok:"+tc.want {
			t.Errorf("GET %s: got %d %q, want 200 %q", tc.in, resp.StatusCode, body, "ok:"+tc.want)
		}
		if resp.Header.Get("Metadata-Flavor") != "Google" || resp.Header.Get("Content-Type") != "application/text" {
			t.Errorf("GET %s: response headers %v, want Metadata-Flavor and Content-Type", tc.in, resp.Header)
		}
		if resp.Header.Get("Server") != "" || resp.Header.Get("ETag") != "" {
			t.Errorf("GET %s: upstream headers leaked: %v", tc.in, resp.Header)
		}
		if len(seen) != before+1 {
			t.Fatalf("GET %s: upstream saw %d requests, want 1", tc.in, len(seen)-before)
		}
		up := seen[before]
		if up.method != http.MethodGet || up.uri != tc.want {
			t.Errorf("GET %s: upstream saw %s %s, want GET %s", tc.in, up.method, up.uri, tc.want)
		}
		if up.headers.Get("Metadata-Flavor") != "Google" {
			t.Errorf("GET %s: Metadata-Flavor not forwarded", tc.in)
		}
		for _, name := range []string{"X-Forwarded-For", "Authorization"} {
			if up.headers.Get(name) != "" {
				t.Errorf("GET %s: header %s forwarded upstream", tc.in, name)
			}
		}
	}

	// The proxy returns an upstream redirect instead of following it.
	before := len(seen)
	if resp, _ := do(http.MethodGet, "/computeMetadata/v1/instance/machine-type"); resp.StatusCode != http.StatusFound {
		t.Errorf("redirecting path: got %d, want 302", resp.StatusCode)
	}
	if len(seen) != before+1 {
		t.Errorf("redirect was followed: upstream saw %q", seen[before:])
	}

	// The proxy refuses an oversized response.
	if resp, _ := do(http.MethodGet, "/computeMetadata/v1/instance/id"); resp.StatusCode != http.StatusBadGateway {
		t.Errorf("oversized response: got %d, want 502", resp.StatusCode)
	}

	// The proxy refuses everything else before it reaches upstream.
	before = len(seen)
	for _, p := range []string{
		"/computeMetadata/v1/instance/service-accounts/default/token",
		"/computeMetadata/v1/instance/service-accounts/default/identity?audience=x",
		"/computeMetadata/v1/instance/service-accounts/default/email",
		"/computeMetadata/v1/instance/service-accounts/",
		"/computeMetadata/v1/instance/attributes/kube-env",
		"/computeMetadata/v1/instance/attributes/",
		"/computeMetadata/v1/instance/",
		"/computeMetadata/v1/instance/?recursive=true",
		"/computeMetadata/v1/",
		"/computeMetadata/v1",
		"/computeMetadata/v1beta1/instance/zone",
		"/0.1/meta-data/service-accounts/default/acquire",
		"/computeMetadata/v1/instance/name",     // known, off by default
		"/computeMetadata/v1/instance/hostname", // known, off by default
		"/computeMetadata/v1/instance/zone/",
		"/computeMetadata/v1/instance/./zone",
		"/computeMetadata/v1/instance/attributes/../zone",
		"/computeMetadata/v1/project/Project-ID",
		"//computeMetadata/v1/instance/zone",
	} {
		if resp, _ := do(http.MethodGet, p); resp.StatusCode != http.StatusForbidden {
			t.Errorf("GET %s: got %d, want 403", p, resp.StatusCode)
		}
	}
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodHead, http.MethodOptions} {
		if resp, _ := do(method, "/computeMetadata/v1/instance/zone"); resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("%s zone: got %d, want 405", method, resp.StatusCode)
		}
	}
	if len(seen) != before {
		t.Errorf("refused requests reached upstream: %q", seen[before:])
	}
}

// TestUpstreamFor checks that the proxy picks the metadata server for the address family the Pod
// connected over, treating IPv4-mapped IPv6 addresses as IPv4.
func TestUpstreamFor(t *testing.T) {
	h, err := newHandler("http://169.254.169.254", "http://[fd20:ce::254]", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		local net.Addr
		want  string
	}{
		{nil, "169.254.169.254"},
		{&net.TCPAddr{IP: net.ParseIP("169.254.169.252"), Port: 988}, "169.254.169.254"},
		{&net.TCPAddr{IP: net.ParseIP("::ffff:169.254.169.252"), Port: 988}, "169.254.169.254"},
		{&net.TCPAddr{IP: net.ParseIP("fd20:ce::252"), Port: 988}, "[fd20:ce::254]"},
		{&net.TCPAddr{IP: net.ParseIP("::1"), Port: 988}, "[fd20:ce::254]"},
	} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if tc.local != nil {
			req = req.WithContext(context.WithValue(req.Context(), http.LocalAddrContextKey, tc.local))
		}
		if got := h.upstreamFor(req).Host; got != tc.want {
			t.Errorf("local %v: got upstream %s, want %s", tc.local, got, tc.want)
		}
	}
}

// TestListen checks that the proxy skips addresses this node does not have, and fails only when it
// cannot bind any of them. 192.0.2.1 is TEST-NET-1 and is never local.
func TestListen(t *testing.T) {
	lns, err := listen(context.Background(), []string{"127.0.0.1:0", " 192.0.2.1:0 ", ""})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	for _, ln := range lns {
		defer ln.Close()
	}
	if len(lns) != 1 || !strings.HasPrefix(lns[0].Addr().String(), "127.0.0.1:") {
		t.Errorf("got listeners %v, want one on 127.0.0.1", lns)
	}

	if _, err := listen(context.Background(), []string{"192.0.2.1:0"}); err == nil {
		t.Error("listen on only unavailable addresses: expected error")
	}
	if _, err := listen(context.Background(), []string{"127.0.0.1:0", "not-an-address"}); err == nil {
		t.Error("listen with a malformed address: expected error")
	}
}

func TestHandlerUpstreamDown(t *testing.T) {
	upstream := httptest.NewServer(http.NotFoundHandler())
	upstream.Close()
	allowed, _ := allowedPaths("")
	h, err := newHandler(upstream.URL, "http://[fd20:ce::254]", allowed)
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/computeMetadata/v1/instance/zone", nil))
	if rec.Code != http.StatusBadGateway {
		t.Errorf("got %d, want 502", rec.Code)
	}
}

func TestNewHandlerRejectsBadUpstream(t *testing.T) {
	for _, u := range []string{"", "169.254.169.254", "://x"} {
		if _, err := newHandler(u, "http://[fd20:ce::254]", nil); err == nil {
			t.Errorf("newHandler(ipv4=%q): expected error", u)
		}
		if _, err := newHandler("http://169.254.169.254", u, nil); err == nil {
			t.Errorf("newHandler(ipv6=%q): expected error", u)
		}
	}
}
