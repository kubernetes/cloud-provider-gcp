/*
Copyright 2018 The Kubernetes Authors.
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

package app

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	compute "google.golang.org/api/compute/v1"
	authorization "k8s.io/api/authorization/v1beta1"
	capi "k8s.io/api/certificates/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	testclient "k8s.io/client-go/testing"
	certutil "k8s.io/kubernetes/pkg/apis/certificates/v1beta1"
)

func TestHasKubeletUsages(t *testing.T) {
	cases := []struct {
		usages   []capi.KeyUsage
		expected bool
	}{
		{
			usages:   nil,
			expected: false,
		},
		{
			usages:   []capi.KeyUsage{},
			expected: false,
		},
		{
			usages: []capi.KeyUsage{
				capi.UsageKeyEncipherment,
				capi.UsageDigitalSignature,
			},
			expected: false,
		},
		{
			usages: []capi.KeyUsage{
				capi.UsageKeyEncipherment,
				capi.UsageDigitalSignature,
				capi.UsageServerAuth,
			},
			expected: false,
		},
		{
			usages: []capi.KeyUsage{
				capi.UsageKeyEncipherment,
				capi.UsageDigitalSignature,
				capi.UsageClientAuth,
			},
			expected: true,
		},
	}
	for _, c := range cases {
		if hasExactUsages(&capi.CertificateSigningRequest{
			Spec: capi.CertificateSigningRequestSpec{
				Usages: c.usages,
			},
		}, kubeletClientUsages) != c.expected {
			t.Errorf("unexpected result of hasKubeletUsages(%v), expecting: %v", c.usages, c.expected)
		}
	}
}

func TestHandle(t *testing.T) {
	verifyCreateAndUpdate := func(t *testing.T, as []testclient.Action) {
		if len(as) != 2 {
			t.Fatalf("expected two calls but got: %#v", as)
		}
		_ = as[0].(testclient.CreateActionImpl)
		a := as[1].(testclient.UpdateActionImpl)
		if got, expected := a.Verb, "update"; got != expected {
			t.Errorf("got: %v, expected: %v", got, expected)
		}
		if got, expected := a.Resource, (schema.GroupVersionResource{Group: "certificates.k8s.io", Version: "v1beta1", Resource: "certificatesigningrequests"}); got != expected {
			t.Errorf("got: %v, expected: %v", got, expected)
		}
		if got, expected := a.Subresource, "approval"; got != expected {
			t.Errorf("got: %v, expected: %v", got, expected)
		}
		csr := a.Object.(*capi.CertificateSigningRequest)
		if len(csr.Status.Conditions) != 1 {
			t.Errorf("expected CSR to have approved condition: %#v", csr)
		}
		c := csr.Status.Conditions[0]
		if got, expected := c.Type, capi.CertificateApproved; got != expected {
			t.Errorf("got: %v, expected: %v", got, expected)
		}
		if got, expected := c.Reason, "AutoApproved"; got != expected {
			t.Errorf("got: %v, expected: %v", got, expected)
		}
	}
	cases := []struct {
		desc          string
		allowed       bool
		recognized    bool
		err           bool
		validate      csrCheckFunc
		verifyActions func(*testing.T, []testclient.Action)
	}{
		{
			desc:       "not recognized not allowed",
			recognized: false,
			verifyActions: func(t *testing.T, as []testclient.Action) {
				if len(as) != 0 {
					t.Errorf("expected no client calls but got: %#v", as)
				}
			},
		},
		{
			desc:       "not recognized but allowed",
			recognized: false,
			allowed:    true,
			verifyActions: func(t *testing.T, as []testclient.Action) {
				if len(as) != 0 {
					t.Errorf("expected no client calls but got: %#v", as)
				}
			},
		},
		{
			desc:       "recognized but not allowed",
			recognized: true,
			allowed:    false,
			verifyActions: func(t *testing.T, as []testclient.Action) {
				if len(as) != 1 {
					t.Fatalf("expected 1 call but got: %#v", as)
				}
				_ = as[0].(testclient.CreateActionImpl)
			},
			err: true,
		},
		{
			desc:          "recognized and allowed",
			recognized:    true,
			allowed:       true,
			verifyActions: verifyCreateAndUpdate,
		},
		{
			desc:       "recognized, allowed and validated",
			recognized: true,
			allowed:    true,
			validate: func(opts GCPConfig, csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) bool {
				return true
			},
			verifyActions: verifyCreateAndUpdate,
		},
		{
			desc:       "recognized, allowed but not validated",
			recognized: true,
			allowed:    true,
			validate: func(opts GCPConfig, csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) bool {
				return false
			},
			verifyActions: func(t *testing.T, as []testclient.Action) {
				if len(as) != 1 {
					t.Fatalf("expected one calls but got: %#v", as)
				}
				a := as[0].(testclient.UpdateActionImpl)
				if got, expected := a.Verb, "update"; got != expected {
					t.Errorf("got: %v, expected: %v", got, expected)
				}
				if got, expected := a.Resource, (schema.GroupVersionResource{Group: "certificates.k8s.io", Version: "v1beta1", Resource: "certificatesigningrequests"}); got != expected {
					t.Errorf("got: %v, expected: %v", got, expected)
				}
				if got, expected := a.Subresource, "approval"; got != expected {
					t.Errorf("got: %v, expected: %v", got, expected)
				}
				csr := a.Object.(*capi.CertificateSigningRequest)
				if len(csr.Status.Conditions) != 1 {
					t.Errorf("expected CSR to have approved condition: %#v", csr)
				}
				c := csr.Status.Conditions[0]
				if got, expected := c.Type, capi.CertificateDenied; got != expected {
					t.Errorf("got: %v, expected: %v", got, expected)
				}
				if got, expected := c.Reason, "AutoDenied"; got != expected {
					t.Errorf("got: %v, expected: %v", got, expected)
				}
			},
		},
	}

	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			client := &fake.Clientset{}
			client.AddReactor("create", "subjectaccessreviews", func(action testclient.Action) (handled bool, ret runtime.Object, err error) {
				return true, &authorization.SubjectAccessReview{
					Status: authorization.SubjectAccessReviewStatus{
						Allowed: c.allowed,
					},
				}, nil
			})
			approver := gkeApprover{
				client: client,
				validators: []csrValidator{
					{
						approveMsg: "tester",
						permission: authorization.ResourceAttributes{Group: "foo", Resource: "bar", Subresource: "baz"},
						recognize: func(opts GCPConfig, csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) bool {
							return c.recognized
						},
						validate: c.validate,
					},
				},
			}
			csr := makeTestCSR()
			if err := approver.handle(csr); err != nil && !c.err {
				t.Errorf("unexpected err: %v", err)
			}
			c.verifyActions(t, client.Actions())
		})
	}
}

func TestValidators(t *testing.T) {
	t.Run("client recognize", func(t *testing.T) {
		goodCases := []func(*csrBuilder, *GCPConfig){
			func(*csrBuilder, *GCPConfig) {},
		}

		testValidator(t, "isNodeClientCert good", goodCases, isNodeClientCert, true)
		testValidator(t, "isSelfNodeClientCert good", goodCases, isSelfNodeClientCert, true)

		badCases := []func(*csrBuilder, *GCPConfig){
			func(b *csrBuilder, _ *GCPConfig) { b.cn = "mike" },
			func(b *csrBuilder, _ *GCPConfig) { b.orgs = nil },
			func(b *csrBuilder, _ *GCPConfig) { b.orgs = []string{"system:master"} },
			func(b *csrBuilder, _ *GCPConfig) { b.usages = kubeletServerUsages },
		}

		testValidator(t, "isNodeClientCert bad", badCases, isNodeClientCert, false)
		testValidator(t, "isSelfNodeClientCert bad", badCases, isSelfNodeClientCert, false)

		// cn different then requestor
		differentCN := []func(*csrBuilder, *GCPConfig){
			func(b *csrBuilder, _ *GCPConfig) { b.requestor = "joe" },
			func(b *csrBuilder, _ *GCPConfig) { b.cn = "system:node:bar" },
		}

		testValidator(t, "isNodeClientCert different CN", differentCN, isNodeClientCert, true)
		testValidator(t, "isSelfNodeClientCert different CN", differentCN, isSelfNodeClientCert, false)
	})
	t.Run("server recognize", func(t *testing.T) {
		goodCases := []func(*csrBuilder, *GCPConfig){
			func(b *csrBuilder, o *GCPConfig) { b.usages = kubeletServerUsages },
		}
		testValidator(t, "isNodeServerClient good", goodCases, isNodeServerCert, true)

		badCases := []func(*csrBuilder, *GCPConfig){
			func(b *csrBuilder, o *GCPConfig) {},
			func(b *csrBuilder, _ *GCPConfig) {
				b.usages = kubeletServerUsages
				b.cn = "mike"
			},
			func(b *csrBuilder, _ *GCPConfig) {
				b.usages = kubeletServerUsages
				b.orgs = nil
			},
			func(b *csrBuilder, _ *GCPConfig) {
				b.usages = kubeletServerUsages
				b.orgs = []string{"system:master"}
			},
			func(b *csrBuilder, _ *GCPConfig) {
				b.usages = kubeletServerUsages
				b.requestor = "joe"
			},
			func(b *csrBuilder, _ *GCPConfig) {
				b.usages = kubeletServerUsages
				b.cn = "system:node:bar"
			},
		}
		testValidator(t, "isNodeServerClient bad", badCases, isNodeServerCert, false)
	})
	t.Run("server validate", func(t *testing.T) {
		fn := func(opts GCPConfig, csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) bool {
			client, srv := fakeGCPAPI(t)
			defer srv.Close()
			err := validateNodeServerCertInner(client, opts, csr, x509cr)
			if err != nil {
				t.Logf("validateNodeServerCertInner: %v", err)
			}
			return err == nil
		}

		cases := []func(*csrBuilder, *GCPConfig){
			func(b *csrBuilder, o *GCPConfig) {
				o.ProjectID = "p0"
				o.Zones = []string{"z1", "z0"}
				b.requestor = "system:node:i0"
				b.ips = []net.IP{net.ParseIP("1.2.3.4")}
			},
		}
		testValidator(t, "validateNodeServerCert good", cases, fn, true)

		cases = []func(*csrBuilder, *GCPConfig){
			func(b *csrBuilder, o *GCPConfig) {},
			// No Name.
			func(b *csrBuilder, o *GCPConfig) {
				o.ProjectID = "p0"
				o.Zones = []string{"z0"}
				b.ips = []net.IP{net.ParseIP("1.2.3.4")}
			},
			// No IPAddresses.
			func(b *csrBuilder, o *GCPConfig) {
				o.ProjectID = "p0"
				o.Zones = []string{"z0"}
				b.requestor = "system:node:i0"
			},
			// Wrong project.
			func(b *csrBuilder, o *GCPConfig) {
				o.ProjectID = "p99"
				o.Zones = []string{"z0"}
				b.requestor = "system:node:i0"
				b.ips = []net.IP{net.ParseIP("1.2.3.4")}
			},
			// Wrong zone.
			func(b *csrBuilder, o *GCPConfig) {
				o.ProjectID = "p0"
				o.Zones = []string{"z99"}
				b.requestor = "system:node:i0"
				b.ips = []net.IP{net.ParseIP("1.2.3.4")}
			},
			// Wrong instance name.
			func(b *csrBuilder, o *GCPConfig) {
				o.ProjectID = "p0"
				o.Zones = []string{"z0"}
				b.requestor = "i99"
				b.ips = []net.IP{net.ParseIP("1.2.3.4")}
			},
			// Not matching IP.
			func(b *csrBuilder, o *GCPConfig) {
				o.ProjectID = "p0"
				o.Zones = []string{"z0"}
				b.requestor = "system:node:i0"
				b.ips = []net.IP{net.ParseIP("1.2.3.5")}
			},
		}
		testValidator(t, "validateNodeServerCert bad", cases, fn, false)
	})
}

func testValidator(t *testing.T, desc string, cases []func(b *csrBuilder, o *GCPConfig), checkFunc csrCheckFunc, want bool) {
	for i, c := range cases {
		b := csrBuilder{
			cn:        "system:node:foo",
			orgs:      []string{"system:nodes"},
			requestor: "system:node:foo",
			usages:    kubeletClientUsages,
		}
		o := GCPConfig{}
		c(&b, &o)
		t.Run(fmt.Sprintf("%s %d", desc, i), func(t *testing.T) {
			csr := makeFancyTestCSR(b)
			x509cr, err := certutil.ParseCSR(csr)
			if err != nil {
				t.Errorf("unexpected err: %v", err)
			}
			if checkFunc(o, csr, x509cr) != want {
				t.Errorf("expected recognized to be %v", want)
			}
		})
	}
}

// noncryptographic for faster testing
// DO NOT COPY THIS CODE
var insecureRand = rand.New(rand.NewSource(0))

func makeTestCSR() *capi.CertificateSigningRequest {
	return makeFancyTestCSR(csrBuilder{cn: "test-cert"})
}

type csrBuilder struct {
	cn        string
	orgs      []string
	requestor string
	usages    []capi.KeyUsage
	dns       []string
	emails    []string
	ips       []net.IP
}

func makeFancyTestCSR(b csrBuilder) *capi.CertificateSigningRequest {
	pk, err := ecdsa.GenerateKey(elliptic.P224(), insecureRand)
	if err != nil {
		panic(err)
	}
	csrb, err := x509.CreateCertificateRequest(insecureRand, &x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName:   b.cn,
			Organization: b.orgs,
		},
		DNSNames:       b.dns,
		EmailAddresses: b.emails,
		IPAddresses:    b.ips,
	}, pk)
	if err != nil {
		panic(err)
	}
	return &capi.CertificateSigningRequest{
		Spec: capi.CertificateSigningRequestSpec{
			Username: b.requestor,
			Usages:   b.usages,
			Request:  pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrb}),
		},
	}
}

func fakeGCPAPI(t *testing.T) (*http.Client, *httptest.Server) {
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		t.Logf("fakeGCPAPI request %q", req.URL.Path)
		switch req.URL.Path {
		case "/compute/v1/projects/p0/zones/z0/instances/i0":
			json.NewEncoder(rw).Encode(compute.Instance{
				Name:              "i0",
				NetworkInterfaces: []*compute.NetworkInterface{{NetworkIP: "1.2.3.4"}},
			})
		case "/compute/v1/projects/p0/zones/z0/instances/i1":
			json.NewEncoder(rw).Encode(compute.Instance{
				Name:              "i1",
				NetworkInterfaces: []*compute.NetworkInterface{{NetworkIP: "1.2.3.5"}},
			})
		default:
			http.Error(rw, "not found", http.StatusNotFound)
		}
	}))
	cl := srv.Client()
	cl.Transport = fakeTransport{srv.URL}
	return cl, srv
}

type fakeTransport struct{ addr string }

func (t fakeTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	u, err := url.Parse(t.addr)
	if err != nil {
		return nil, err
	}
	r.URL.Scheme = u.Scheme
	r.URL.Host = u.Host
	return http.DefaultClient.Do(r)
}
