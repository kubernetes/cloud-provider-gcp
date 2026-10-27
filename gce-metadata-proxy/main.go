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

// gce-metadata-proxy gives Pods a small, fixed set of Compute Engine metadata server endpoints.
//
// It runs on every node, on the host network, and listens on addresses that node provisioning
// redirects Pod traffic for the metadata server to (169.254.169.252:988 for IPv4 and
// [fd20:ce::252]:988 for IPv6). When a Pod sends a GET for an allowed path, the proxy fetches the
// value from the real metadata server for the same address family and returns it. It refuses
// everything else.
//
// The paths it can allow describe where the Pod is running (project, zone, instance). It never
// serves service-account tokens, identity documents or instance attributes: Pods that need Google
// Cloud credentials should use Workload Identity Federation with Kubernetes.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"maps"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"k8s.io/klog/v2"
)

const (
	apiPrefix = "/computeMetadata/v1/"

	// pathsEnv names the environment variable that chooses which of knownPaths the proxy serves:
	// a comma-separated list of "+path" to enable and "-path" to disable, relative to apiPrefix.
	pathsEnv = "GCE_METADATA_PROXY_PATHS"

	// maxResponseBytes caps an upstream response. The allowed values are short strings; the cap
	// leaves plenty of room for the listing at the root.
	maxResponseBytes = 64 << 10

	upstreamTimeout = 5 * time.Second
)

// knownPaths lists the metadata-server paths, relative to apiPrefix, that the proxy can serve, and
// whether it serves each by default. Operators can only enable paths in this map; adding one is a
// code change on purpose.
var knownPaths = map[string]bool{
	"project/project-id":         true,
	"project/numeric-project-id": true,
	"instance/zone":              true,

	// These describe the node rather than the Pod. Telemetry agents use them to label data by
	// host, but most workloads have no use for them, so they stay off unless an operator enables
	// them.
	"instance/id":           false,
	"instance/name":         false,
	"instance/hostname":     false,
	"instance/machine-type": false,
}

// version comes from -ldflags "-X main.version=..." at build time.
var version = "dev"

func main() {
	klog.InitFlags(nil)
	var (
		addrs        = flag.String("addr", "169.254.169.252:988,[fd20:ce::252]:988", "Comma-separated addresses to listen on. The proxy skips an address this node does not have and fails only if it can bind none of them.")
		upstreamIPv4 = flag.String("upstream-ipv4", "http://169.254.169.254", "Metadata server for requests that arrive over IPv4")
		upstreamIPv6 = flag.String("upstream-ipv6", "http://[fd20:ce::254]", "Metadata server for requests that arrive over IPv6")
	)
	flag.Parse()

	allowed, err := allowedPaths(os.Getenv(pathsEnv))
	if err != nil {
		klog.Fatal(err)
	}
	handler, err := newHandler(*upstreamIPv4, *upstreamIPv6, allowed)
	if err != nil {
		klog.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	listeners, err := listen(ctx, strings.Split(*addrs, ","))
	if err != nil {
		klog.Fatal(err)
	}
	var listening []string
	for _, ln := range listeners {
		listening = append(listening, ln.Addr().String())
	}
	klog.Infof("gce-metadata-proxy %s listening on %s, serving %s", version, strings.Join(listening, " and "),
		strings.Join(slices.Sorted(maps.Keys(allowed)), " "))
	if err := serve(ctx, listeners, handler); err != nil {
		klog.Fatal(err)
	}
}

// allowedPaths applies a pathsEnv specification such as "+instance/machine-type,-project/numeric-project-id"
// to the defaults in knownPaths and returns the full set of request paths the proxy serves. It
// always includes the root: client libraries fetch it to detect that they are on GCE, by checking
// for the Metadata-Flavor: Google response header.
func allowedPaths(spec string) (map[string]bool, error) {
	enabled := maps.Clone(knownPaths)
	for _, item := range strings.Split(spec, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if len(item) < 2 || (item[0] != '+' && item[0] != '-') {
			return nil, fmt.Errorf("%s: entry %q must be +path or -path", pathsEnv, item)
		}
		p := item[1:]
		if _, ok := knownPaths[p]; !ok {
			return nil, fmt.Errorf("%s: unknown path %q (known paths: %s)", pathsEnv, p,
				strings.Join(slices.Sorted(maps.Keys(knownPaths)), ", "))
		}
		enabled[p] = item[0] == '+'
	}

	allowed := map[string]bool{"/": true}
	for p, on := range enabled {
		if on {
			allowed[apiPrefix+p] = true
		}
	}
	return allowed, nil
}

// handler fetches allowed paths from the metadata server on behalf of Pods.
type handler struct {
	upstreamIPv4 *url.URL
	upstreamIPv6 *url.URL
	allowed      map[string]bool
	client       *http.Client
}

func newHandler(upstreamIPv4, upstreamIPv6 string, allowed map[string]bool) (*handler, error) {
	u4, err := parseUpstream(upstreamIPv4)
	if err != nil {
		return nil, err
	}
	u6, err := parseUpstream(upstreamIPv6)
	if err != nil {
		return nil, err
	}
	return &handler{
		upstreamIPv4: u4,
		upstreamIPv6: u6,
		allowed:      allowed,
		client: &http.Client{
			Timeout: upstreamTimeout,
			// Not http.DefaultTransport: that honours HTTP_PROXY, and metadata requests must never
			// leave the node.
			Transport: &http.Transport{
				Proxy:           nil,
				DialContext:     (&net.Dialer{Timeout: upstreamTimeout}).DialContext,
				MaxIdleConns:    16,
				IdleConnTimeout: 90 * time.Second,
			},
			// Return redirects to the Pod rather than following them to a path the proxy has not checked.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}, nil
}

func parseUpstream(upstream string) (*url.URL, error) {
	u, err := url.Parse(upstream)
	if err != nil {
		return nil, fmt.Errorf("parsing upstream %q: %w", upstream, err)
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("upstream %q must be a URL with a scheme and host", upstream)
	}
	return u, nil
}

// upstreamFor picks the metadata server that matches the address family the Pod connected over, so
// that a Pod talking IPv4 reaches 169.254.169.254 and a Pod talking IPv6 reaches fd20:ce::254.
func (h *handler) upstreamFor(req *http.Request) *url.URL {
	if local, ok := req.Context().Value(http.LocalAddrContextKey).(*net.TCPAddr); ok && local.IP.To4() == nil {
		return h.upstreamIPv6
	}
	return h.upstreamIPv4
}

// ServeHTTP serves GET requests for allowed paths. It sends exactly one request header upstream
// (Metadata-Flavor, which the server requires), returns exactly two (Content-Type and
// Metadata-Flavor, which client libraries check to detect GCE), and drops the query string.
func (h *handler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		klog.V(2).Infof("refused %s %s: method not allowed", req.Method, req.URL.Path)
		http.Error(w, "gce-metadata-proxy only serves GET requests", http.StatusMethodNotAllowed)
		return
	}
	if !h.allowed[req.URL.Path] {
		klog.V(2).Infof("refused GET %s: not an allowed path", req.URL.Path)
		http.Error(w, "gce-metadata-proxy does not serve this metadata path to Pods", http.StatusForbidden)
		return
	}

	target := *h.upstreamFor(req)
	target.Path = req.URL.Path
	target.RawPath = ""
	target.RawQuery = ""
	out, err := http.NewRequestWithContext(req.Context(), http.MethodGet, target.String(), nil)
	if err != nil {
		klog.Errorf("building upstream request for %s: %v", req.URL.Path, err)
		http.Error(w, "gce-metadata-proxy could not build the upstream request", http.StatusInternalServerError)
		return
	}
	if v := req.Header.Get("Metadata-Flavor"); v != "" {
		out.Header.Set("Metadata-Flavor", v)
	}

	resp, err := h.client.Do(out)
	if err != nil {
		klog.Warningf("GET %s: upstream request failed: %v", req.URL.Path, err)
		http.Error(w, "gce-metadata-proxy could not reach the metadata server", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		klog.Warningf("GET %s: reading upstream response: %v", req.URL.Path, err)
		http.Error(w, "gce-metadata-proxy could not read the metadata server's response", http.StatusBadGateway)
		return
	}
	if len(body) > maxResponseBytes {
		klog.Warningf("GET %s: upstream response exceeds %d bytes", req.URL.Path, maxResponseBytes)
		http.Error(w, "the metadata server returned a response that is too large", http.StatusBadGateway)
		return
	}

	klog.V(4).Infof("GET %s: %d", req.URL.Path, resp.StatusCode)
	for _, name := range []string{"Content-Type", "Metadata-Flavor"} {
		if v := resp.Header.Get(name); v != "" {
			w.Header().Set(name, v)
		}
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(body)
}

// listen opens a listener on each address. Node provisioning adds the listen addresses to the
// loopback interface, and a single-stack node only has one of them, so listen skips an address the
// kernel says this node does not have (or whose address family the kernel does not support) and
// fails only when it cannot bind any address at all. Skipping is safe: Pod connections that the
// node redirects to an address nobody listens on are refused, not passed through.
func listen(ctx context.Context, addrs []string) ([]net.Listener, error) {
	lc := net.ListenConfig{KeepAlive: 3 * time.Minute}
	var listeners []net.Listener
	for _, addr := range addrs {
		addr = strings.TrimSpace(addr)
		if addr == "" {
			continue
		}
		ln, err := lc.Listen(ctx, "tcp", addr)
		if err != nil {
			if errors.Is(err, syscall.EADDRNOTAVAIL) || errors.Is(err, syscall.EAFNOSUPPORT) {
				klog.Infof("not listening on %s: this node does not have the address (%v)", addr, err)
				continue
			}
			for _, ln := range listeners {
				ln.Close()
			}
			return nil, fmt.Errorf("listening on %s: %w", addr, err)
		}
		listeners = append(listeners, ln)
	}
	if len(listeners) == 0 {
		return nil, fmt.Errorf("could not listen on any of %s", strings.Join(addrs, ", "))
	}
	return listeners, nil
}

// serve serves handler on every listener until the caller cancels ctx, then drains open connections
// and returns.
func serve(ctx context.Context, listeners []net.Listener, handler http.Handler) error {
	s := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       90 * time.Second,
		MaxHeaderBytes:    64 << 10,
	}

	errCh := make(chan error, len(listeners))
	for _, ln := range listeners {
		go func() { errCh <- s.Serve(ln) }()
	}
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		// The caller cancelled ctx. Detach from that cancellation but keep ctx's values for the shutdown.
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		if err := s.Shutdown(shutdownCtx); err != nil && !errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		return nil
	}
}
