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

package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	betacompute "google.golang.org/api/compute/v0.beta"
	compute "google.golang.org/api/compute/v1"
	authorization "k8s.io/api/authorization/v1"
	capi "k8s.io/api/certificates/v1"
	certsv1 "k8s.io/api/certificates/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes/fake"
	testclient "k8s.io/client-go/testing"
	"k8s.io/klog/v2"
	certutil "k8s.io/kubernetes/pkg/apis/certificates/v1"
	"k8s.io/utils/pointer"
)

func init() {
	// Make sure we get all useful output in stderr.
	klog.SetOutput(os.Stderr)
}

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

func TestNodeApproverHandle(t *testing.T) {
	verifyCreateAndUpdate := func(t *testing.T, as []testclient.Action) {
		if len(as) != 2 {
			t.Fatalf("expected two calls but got: %#v", as)
		}
		_ = as[0].(testclient.CreateActionImpl)
		a := as[1].(testclient.UpdateActionImpl)
		if got, expected := a.Verb, "update"; got != expected {
			t.Errorf("got: %v, expected: %v", got, expected)
		}
		if got, expected := a.Resource, (schema.GroupVersionResource{Group: "certificates.k8s.io", Version: "v1", Resource: "certificatesigningrequests"}); got != expected {
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
		desc           string
		allowed        bool
		recognized     bool
		err            bool
		validate       validateFunc
		verifyActions  func(*testing.T, []testclient.Action)
		preApproveHook preApproveHookFunc
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
			desc:          "recognized, allowed and passed preApproveHook",
			recognized:    true,
			allowed:       true,
			verifyActions: verifyCreateAndUpdate,
			preApproveHook: func(_ *controllerContext, _ *capi.CertificateSigningRequest, _ *x509.CertificateRequest) error {
				return nil
			},
		},
		{
			desc:       "recognized, allowed but failed preApproveHook",
			recognized: true,
			allowed:    true,
			verifyActions: func(t *testing.T, as []testclient.Action) {
				if len(as) != 1 {
					t.Fatalf("expected 1 call but got: %#v", as)
				}
				_ = as[0].(testclient.CreateActionImpl)
			},
			preApproveHook: func(_ *controllerContext, _ *capi.CertificateSigningRequest, _ *x509.CertificateRequest) error {
				return fmt.Errorf("preApproveHook failed")
			},
			err: true,
		},
		{
			desc:       "recognized, allowed and validated",
			recognized: true,
			allowed:    true,
			validate: func(ctx *controllerContext, csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) (bool, error) {
				return true, nil
			},
			verifyActions: verifyCreateAndUpdate,
		},
		{
			desc:       "recognized, allowed but not validated",
			recognized: true,
			allowed:    true,
			validate: func(ctx *controllerContext, csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) (bool, error) {
				return false, nil
			},
			verifyActions: func(t *testing.T, as []testclient.Action) {
				if len(as) != 1 {
					t.Fatalf("expected one calls but got: %#v", as)
				}
				a := as[0].(testclient.UpdateActionImpl)
				if got, expected := a.Verb, "update"; got != expected {
					t.Errorf("got: %v, expected: %v", got, expected)
				}
				if got, expected := a.Resource, (schema.GroupVersionResource{Group: "certificates.k8s.io", Version: "v1", Resource: "certificatesigningrequests"}); got != expected {
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
		{
			desc:       "recognized, allowed but validation failed",
			recognized: true,
			allowed:    true,
			validate: func(ctx *controllerContext, csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) (bool, error) {
				return false, errors.New("failed")
			},
			verifyActions: func(t *testing.T, as []testclient.Action) {
				if len(as) != 0 {
					t.Fatalf("expected no calls but got: %#v", as)
				}
			},
			err: true,
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
			validator := csrValidator{
				approveMsg: "tester",
				permission: authorization.ResourceAttributes{Group: "foo", Resource: "bar", Subresource: "baz"},
				recognize: func(csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) bool {
					return c.recognized
				},
				validate:       c.validate,
				preApproveHook: c.preApproveHook,
			}
			approver := nodeApprover{
				ctx:        &controllerContext{client: client},
				validators: []csrValidator{validator},
			}
			csr := makeTestCSR(t)
			if err := approver.handle(context.TODO(), csr); err != nil && !c.err {
				t.Errorf("unexpected err: %v", err)
			}
			c.verifyActions(t, client.Actions())
		})
	}
}

func TestValidators(t *testing.T) {
	t.Run("isLegacyNodeClientCert", func(t *testing.T) {
		goodCase := func(b *csrBuilder, _ *controllerContext) {
			b.requestor = legacyKubeletUsername
			b.signerName = certsv1.KubeAPIServerClientKubeletSignerName
		}
		goodCases := []func(*csrBuilder, *controllerContext){goodCase}
		testRecognizer(t, "good", goodCases, isLegacyNodeClientCert, true)

		badCases := []func(*csrBuilder, *controllerContext){
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				b.signerName = "" // Should not recognize "" signer name
			},
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				b.signerName = certsv1.KubeletServingSignerName // Should not recognize other signer name
			},
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				b.cn = "mike"
			},
			func(b *csrBuilder, _ *controllerContext) {},
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				b.orgs = nil
			},
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				b.orgs = []string{"system:master"}
			},
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				b.usages = kubeletServerUsages
			},
		}
		testRecognizer(t, "bad", badCases, isLegacyNodeClientCert, false)
	})
	t.Run("isNodeServerCert", func(t *testing.T) {
		goodCase := func(b *csrBuilder, _ *controllerContext) {
			b.usages = kubeletServerUsages
			b.signerName = certsv1.KubeletServingSignerName
		}
		goodCases := []func(*csrBuilder, *controllerContext){goodCase}
		testRecognizer(t, "good", goodCases, isNodeServerCert, true)

		badCases := []func(*csrBuilder, *controllerContext){
			func(b *csrBuilder, c *controllerContext) {},
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				b.signerName = "" // Should not recognize "" signer name
			},
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				b.signerName = certsv1.KubeAPIServerClientKubeletSignerName // Should not recognize other signer name
			},
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				b.cn = "mike"
			},
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				b.orgs = nil
			},
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				b.orgs = []string{"system:master"}
			},
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				b.requestor = "joe"
			},
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				b.cn = "system:node:bar"
			},
		}
		testRecognizer(t, "bad", badCases, isNodeServerCert, false)
	})
	t.Run("validateNodeServerCertInner", func(t *testing.T) {
		client, srv := fakeGCPAPI(t, nil)
		defer srv.Close()
		fn := func(ctx *controllerContext, csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) (bool, error) {
			cs, err := compute.New(client)
			if err != nil {
				t.Fatalf("creating GCE API client: %v", err)
			}
			ctx.gcpCfg.Compute = cs
			return validateNodeServerCert(ctx, csr, x509cr)
		}

		goodCase := func(b *csrBuilder, c *controllerContext) {
			c.gcpCfg.ProjectID = "p0"
			c.gcpCfg.Zones = []string{"z1", "z0"}
			b.requestor = "system:node:i0"
			b.ips = []net.IP{net.ParseIP("1.2.3.4")}
			b.dns = []string{"i0.z0.c.p0.internal", "i0.c.p0.internal", "i0"}
		}
		dualStackCase := func(b *csrBuilder, c *controllerContext) {
			c.gcpCfg.ProjectID = "p0"
			c.gcpCfg.Zones = []string{"z1", "z0"}
			b.requestor = "system:node:ds0"
			b.ips = []net.IP{net.ParseIP("1.2.3.4"), net.ParseIP("fd20:fbc:b0e2::b:0:0")}
			b.dns = []string{"ds0.z0.c.p0.internal", "ds0.c.p0.internal", "ds0"}
		}
		dualStackExtCase := func(b *csrBuilder, c *controllerContext) {
			c.gcpCfg.ProjectID = "p0"
			c.gcpCfg.Zones = []string{"z1", "z0"}
			b.requestor = "system:node:ds1"
			b.ips = []net.IP{net.ParseIP("1.2.3.4"), net.ParseIP("2600:1900:1:1:0:5::")}
			b.dns = []string{"ds1.z0.c.p0.internal", "ds1.c.p0.internal", "ds1"}
		}
		cases := []func(*csrBuilder, *controllerContext){
			// None Domain-scoped project
			goodCase,
			dualStackCase,
			dualStackExtCase,
			// Domain-scoped project
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				c.gcpCfg.ProjectID = "p0:p1"
				b.dns = []string{"i0.z0.c.p1.p0.internal", "i0.c.p1.p0.internal", "i0"}
			},
		}
		testValidator(t, "good", cases, fn, true, false)

		cases = []func(*csrBuilder, *controllerContext){
			func(b *csrBuilder, c *controllerContext) {},
			// No Name.
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				b.requestor = ""
			},
			// No IPAddresses.
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				b.ips = nil
			},
			// Wrong project.
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				c.gcpCfg.ProjectID = "p99"
			},
			// Wrong zone.
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				c.gcpCfg.Zones = []string{"z99"}
			},
			// Wrong instance name.
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				b.requestor = "i99"
			},
			// Not matching IP.
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				b.ips = []net.IP{net.ParseIP("1.2.3.5")}
			},
			// Not matching zonal DNS.
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				b.dns = []string{"i1.z0.c.p1.internal", "i0.c.p0.internal", "i0"}
			},
			// Not matching global DNS.
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				b.dns = []string{"i0.z0.c.p0.internal", "i1.c.p1.internal", "i0"}
			},
			// Not matching windows DNS.
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				b.dns = []string{"i0.z0.c.p0.internal", "i0.c.p0.internal", "i1"}
			},
			// Not matching Domain-scoped project DNS.
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				c.gcpCfg.ProjectID = "p0:p1:p2"
				b.dns = []string{"i0.z0.c.p0.internal", "i0.c.p1.p2.p0.internal", "i0"}
			},
		}
		testValidator(t, "bad", cases, fn, false, false)
	})
}

func testRecognizer(t *testing.T, desc string, cases []func(b *csrBuilder, c *controllerContext), recognize recognizeFunc, want bool) {
	forAllCases(t, desc, cases, func(t *testing.T, _ *controllerContext, csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) {
		got := recognize(csr, x509cr)
		if got != want {
			t.Errorf("got: %v, want: %v", got, want)
		}
	})
}

func testValidator(t *testing.T, desc string, cases []func(b *csrBuilder, c *controllerContext), validate validateFunc, wantOK, wantErr bool) {
	forAllCases(t, desc, cases, func(t *testing.T, ctx *controllerContext, csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) {
		gotOK, gotErr := validate(ctx, csr, x509cr)
		if (gotErr != nil) != wantErr {
			t.Fatalf("got error: %v, want error: %v", gotErr, wantErr)
		}
		if gotOK != wantOK {
			t.Errorf("got: %v, want: %v", gotOK, wantOK)
		}
	})
}

type checkFunc func(t *testing.T, ctx *controllerContext, csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest)

func forAllCases(t *testing.T, desc string, cases []func(b *csrBuilder, c *controllerContext), check checkFunc) {
	t.Helper()
	for i, c := range cases {
		pk, err := ecdsa.GenerateKey(elliptic.P224(), insecureRand)
		if err != nil {
			t.Fatal(err)
		}
		b := csrBuilder{
			cn:        "system:node:foo",
			orgs:      []string{"system:nodes"},
			requestor: "system:node:foo",
			usages:    kubeletClientUsages,
			extraPEM:  make(map[string][]byte),
			key:       pk,
		}
		o := &controllerContext{}
		c(&b, o)
		t.Run(fmt.Sprintf("%s %d", desc, i), func(t *testing.T) {
			csr := makeFancyTestCSR(t, b)
			csr.Name = t.Name()
			x509cr, err := certutil.ParseCSR(csr.Spec.Request)
			if err != nil {
				t.Errorf("unexpected err: %v", err)
			}
			check(t, o, csr, x509cr)
		})
	}
}

// noncryptographic for faster testing
// DO NOT COPY THIS CODE
var insecureRand = rand.New(rand.NewSource(0))

func makeTestCSR(t *testing.T) *capi.CertificateSigningRequest {
	pk, err := ecdsa.GenerateKey(elliptic.P224(), insecureRand)
	if err != nil {
		t.Fatal(err)
	}
	return makeFancyTestCSR(t, csrBuilder{cn: "test-cert", key: pk})
}

type csrBuilder struct {
	cn         string
	orgs       []string
	requestor  string
	signerName string
	usages     []capi.KeyUsage
	dns        []string
	emails     []string
	ips        []net.IP
	extraPEM   map[string][]byte
	key        *ecdsa.PrivateKey
}

func makeFancyTestCSR(t *testing.T, b csrBuilder) *capi.CertificateSigningRequest {
	csrb, err := x509.CreateCertificateRequest(insecureRand, &x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName:   b.cn,
			Organization: b.orgs,
		},
		DNSNames:       b.dns,
		EmailAddresses: b.emails,
		IPAddresses:    b.ips,
	}, b.key)
	if err != nil {
		t.Fatal(err)
	}
	blocks := [][]byte{pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrb})}
	for typ, data := range b.extraPEM {
		blocks = append(blocks, pem.EncodeToMemory(&pem.Block{Type: typ, Bytes: data}))
	}
	return &capi.CertificateSigningRequest{
		Spec: capi.CertificateSigningRequestSpec{
			Username:   b.requestor,
			Usages:     b.usages,
			Request:    bytes.TrimSpace(bytes.Join(blocks, nil)),
			SignerName: b.signerName,
		},
	}
}

func fakeGCPAPI(t *testing.T, ekPub *rsa.PublicKey) (*http.Client, *httptest.Server) {
	var ekPubPEM []byte
	if ekPub != nil {
		ekPubRaw, err := x509.MarshalPKIXPublicKey(ekPub)
		if err != nil {
			t.Fatal(err)
		}
		ekPubPEM = pem.EncodeToMemory(&pem.Block{
			Type:  "PUBLIC KEY",
			Bytes: ekPubRaw,
		})
	}

	var computeMetadata = func(s string) *compute.Metadata {
		return &compute.Metadata{
			Items: []*compute.MetadataItems{
				{Key: "created-by", Value: &s},
			},
		}
	}

	var formatInstanceZone = func(prj string, zone string) string {
		return fmt.Sprintf("https://www.googleapis.com/compute/v1/projects/%s/zones/%s", prj, zone)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		t.Logf("fakeGCPAPI request %q", req.URL.Path)
		switch req.URL.Path {
		case "/compute/v1/projects/p0/zones/z0/instances/i0":
			json.NewEncoder(rw).Encode(compute.Instance{
				Id:                1,
				Name:              "i0",
				Zone:              formatInstanceZone("p0", "z0"),
				Metadata:          computeMetadata("invalid-instance-group"),
				NetworkInterfaces: []*compute.NetworkInterface{{NetworkIP: "1.2.3.4"}},
			})
		case "/compute/v1/projects/p0:p1/zones/z0/instances/i0":
			json.NewEncoder(rw).Encode(compute.Instance{
				Id:                1,
				Name:              "i0",
				Zone:              formatInstanceZone("p0", "z0"),
				NetworkInterfaces: []*compute.NetworkInterface{{NetworkIP: "1.2.3.4"}},
			})
		case "/compute/v1/projects/p0/zones/z0/instances/i1":
			json.NewEncoder(rw).Encode(compute.Instance{
				Id:                2,
				Name:              "i1",
				Zone:              formatInstanceZone("p0", "z0"),
				NetworkInterfaces: []*compute.NetworkInterface{{NetworkIP: "1.2.3.5"}},
			})
		case "/compute/v1/projects/2/zones/z0/instances/i0":
			json.NewEncoder(rw).Encode(compute.Instance{
				Id:                3,
				Name:              "i0",
				Zone:              formatInstanceZone("2", "z0"),
				Metadata:          computeMetadata("invalid-instance-group-3"),
				NetworkInterfaces: []*compute.NetworkInterface{{NetworkIP: "1.2.3.4"}},
			})
		case "/compute/v1/projects/2/zones/r0-a/instances/i1":
			json.NewEncoder(rw).Encode(compute.Instance{
				Id:                4,
				Name:              "i0",
				Zone:              formatInstanceZone("2", "r0-a"),
				Metadata:          computeMetadata("projects/2/zones/r0-a/instanceGroupManagers/ig1"),
				NetworkInterfaces: []*compute.NetworkInterface{{NetworkIP: "1.2.3.4"}},
			})
		case "/compute/beta/projects/2/zones/z0/instances/i0/getShieldedVmIdentity":
			json.NewEncoder(rw).Encode(betacompute.ShieldedVmIdentity{
				SigningKey: &betacompute.ShieldedVmIdentityEntry{
					EkPub: string(ekPubPEM),
				},
			})
		case "/compute/beta/projects/2/zones/r0-a/instances/i1/getShieldedVmIdentity":
			json.NewEncoder(rw).Encode(betacompute.ShieldedVmIdentity{
				SigningKey: &betacompute.ShieldedVmIdentityEntry{
					EkPub: string(ekPubPEM),
				},
			})
		case "/compute/v1/projects/p0/zones/z0/instanceGroupManagers/ig0/listManagedInstances":
			json.NewEncoder(rw).Encode(compute.InstanceGroupManagersListManagedInstancesResponse{
				ManagedInstances: []*compute.ManagedInstance{{
					Id: 3,
				}},
			})
		case "/compute/v1/projects/p0/zones/z0/instanceGroupManagers/ig1/listManagedInstances":
			json.NewEncoder(rw).Encode(compute.InstanceGroupManagersListManagedInstancesResponse{
				ManagedInstances: []*compute.ManagedInstance{{
					Id: 4,
				}},
			})
		case "/compute/v1/projects/p0/zones/r0/instanceGroupManagers/ig0/listManagedInstances":
			json.NewEncoder(rw).Encode(compute.InstanceGroupManagersListManagedInstancesResponse{
				ManagedInstances: []*compute.ManagedInstance{{
					Id: 4,
				}},
			})
		case "/compute/v1/projects/p0/zones/z0/instances/ds0":
			json.NewEncoder(rw).Encode(compute.Instance{
				Id:                1,
				Name:              "ds0",
				Zone:              "z0",
				NetworkInterfaces: []*compute.NetworkInterface{{NetworkIP: "1.2.3.4", Ipv6Address: "fd20:fbc:b0e2:0:0:b:0:0"}},
			})
		case "/compute/v1/projects/p0/zones/z0/instances/ds1":
			json.NewEncoder(rw).Encode(compute.Instance{
				Id:   1,
				Name: "ds1",
				Zone: "z0",
				NetworkInterfaces: []*compute.NetworkInterface{
					{
						NetworkIP: "1.2.3.4",
						Ipv6AccessConfigs: []*compute.AccessConfig{
							{
								ExternalIpv6: "2600:1900:1:1:0:5::",
							},
						},
					},
				},
			})
		case "/compute/v1/projects/p0/zones/z0/instances/i0/referrers":
			json.NewEncoder(rw).Encode(compute.InstanceListReferrers{
				Items: []*compute.Reference{
					{
						Referrer: "https://www.googleapis.com/compute/v1/projects/2/zones/z0/instanceGroups/ig0",
					},
				},
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

func TestShouldDeleteNode(t *testing.T) {
	testErr := fmt.Errorf("intended error")
	cases := []struct {
		desc           string
		ctx            *controllerContext
		node           *v1.Node
		instance       *compute.Instance
		shouldDelete   bool
		getInstanceErr error
		expectedErr    error
	}{
		{
			desc: "instance with 2 alias range and 1 matches podCIDR",
			ctx:  &controllerContext{},
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node-test",
				},
				Spec: v1.NodeSpec{
					PodCIDR: "10.0.0.1/24",
				},
			},
			instance: &compute.Instance{
				NetworkInterfaces: []*compute.NetworkInterface{
					{
						AliasIpRanges: []*compute.AliasIpRange{
							{
								IpCidrRange: "10.0.0.2/24",
							},
							{
								IpCidrRange: "10.0.0.1/24",
							},
						},
					},
				},
			},
		},
		{
			desc: "instance with 0 alias range",
			ctx:  &controllerContext{},
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node-test",
				},
				Spec: v1.NodeSpec{
					PodCIDR: "10.0.0.1/24",
				},
			},
			instance: &compute.Instance{},
		},
		{
			desc: "node with empty podCIDR",
			ctx:  &controllerContext{},
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node-test",
				},
			},
			instance: &compute.Instance{},
		},
		{
			desc: "instance not found",
			ctx:  &controllerContext{},
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node-test",
				},
				Spec: v1.NodeSpec{
					PodCIDR: "10.0.0.1/24",
				},
			},
			shouldDelete:   true,
			getInstanceErr: errInstanceNotFound,
		},
		{
			desc: "error gettting instance",
			ctx:  &controllerContext{},
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node-test",
				},
				Spec: v1.NodeSpec{
					PodCIDR: "10.0.0.1/24",
				},
			},
			getInstanceErr: testErr,
			expectedErr:    testErr,
		},
		{
			desc: "node with different instance id",
			ctx: &controllerContext{
				clearStalePodsOnNodeRegistration: true,
			},
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node-test",
					Annotations: map[string]string{
						InstanceIDAnnotationKey: "1234567890123456789",
					},
				},
				Spec: v1.NodeSpec{
					PodCIDR: "10.0.0.1/24",
				},
			},
			instance: &compute.Instance{
				Id: 0,
			},
			shouldDelete: true,
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			fakeGetInstance := func(_ *controllerContext, _ string) (*compute.Instance, error) {
				return c.instance, c.getInstanceErr
			}
			shouldDelete, err := shouldDeleteNode(c.ctx, c.node, fakeGetInstance)
			if err != c.expectedErr || shouldDelete != c.shouldDelete {
				t.Errorf("%s: shouldDeleteNode=(%v, %v), want (%v, %v)", c.desc, shouldDelete, err, c.shouldDelete, c.expectedErr)
			}
		})
	}
}

func TestDeleteAllPodsBoundToNode(t *testing.T) {
	testNode := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "testNode"},
		Status: v1.NodeStatus{
			Capacity: v1.ResourceList{
				v1.ResourceName(v1.ResourceCPU):    resource.MustParse("10"),
				v1.ResourceName(v1.ResourceMemory): resource.MustParse("10G"),
			},
		},
	}

	for _, tc := range []struct {
		desc                 string
		node                 *v1.Node
		pod                  *v1.Pod
		expectedPatchedPod   *v1.Pod
		expectedDeleteAction *testclient.DeleteActionImpl
	}{
		{
			desc: "Pod that was Running should enter Failed phase prior to deletion",
			node: testNode,
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "testPod",
				},
				Spec: v1.PodSpec{
					NodeName: "testNode",
				},
				Status: v1.PodStatus{
					Phase: v1.PodRunning,
					Conditions: []v1.PodCondition{
						{
							Type:   v1.PodReady,
							Status: v1.ConditionTrue,
						},
					},
				},
			},
			expectedPatchedPod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "testPod",
				},
				Spec: v1.PodSpec{
					NodeName: "testNode",
				},
				Status: v1.PodStatus{
					Phase: v1.PodFailed,
					Conditions: []v1.PodCondition{
						{
							Type:   v1.PodReady,
							Status: v1.ConditionTrue,
						},
						{
							Type:    v1.DisruptionTarget,
							Status:  v1.ConditionTrue,
							Reason:  "DeletionByGCPControllerManager",
							Message: "GCPControllerManager: node no longer exists",
						},
					},
				},
			},
			expectedDeleteAction: &testclient.DeleteActionImpl{Name: "testPod", DeleteOptions: metav1.DeleteOptions{GracePeriodSeconds: pointer.Int64(0)}},
		},
		{
			desc: "Pod that was in Failed phase should remain in Failed phase prior to deletion. Pod should not be patched because it is already in terminal phase.",
			node: testNode,
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "testPod",
				},
				Spec: v1.PodSpec{
					NodeName: "testNode",
				},
				Status: v1.PodStatus{
					Phase: v1.PodFailed,
					Conditions: []v1.PodCondition{
						{
							Type:   v1.PodReady,
							Status: v1.ConditionTrue,
						},
					},
				},
			},
			expectedPatchedPod:   nil,
			expectedDeleteAction: &testclient.DeleteActionImpl{Name: "testPod", DeleteOptions: metav1.DeleteOptions{GracePeriodSeconds: pointer.Int64(0)}},
		},
		{
			desc: "Pod that was in Succeeded phase should remain in Succeeded phase prior to deletion. Pod should not be patched because it is already in terminal phase.",
			node: testNode,
			pod: &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
					Name:      "testPod",
				},
				Spec: v1.PodSpec{
					NodeName: "testNode",
				},
				Status: v1.PodStatus{
					Phase: v1.PodSucceeded,
					Conditions: []v1.PodCondition{
						{
							Type:   v1.PodReady,
							Status: v1.ConditionTrue,
						},
					},
				},
			},
			expectedPatchedPod:   nil,
			expectedDeleteAction: &testclient.DeleteActionImpl{Name: "testPod", DeleteOptions: metav1.DeleteOptions{GracePeriodSeconds: pointer.Int64(0)}},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			nodeList := &v1.NodeList{Items: []v1.Node{*tc.node}}
			podList := &v1.PodList{Items: []v1.Pod{*tc.pod}}

			client := fake.NewSimpleClientset(nodeList, podList)
			controllerCtx := &controllerContext{client: client}

			err := deleteAllPodsBoundToNode(controllerCtx, tc.node.Name)

			if err != nil {
				t.Fatalf("Unexpected error deleting all pods bound to node %v", err)
			}

			actions := client.Actions()

			expectedNumberOfActions := 0
			if tc.expectedPatchedPod != nil {
				expectedNumberOfActions++
			}
			if tc.expectedDeleteAction != nil {
				expectedNumberOfActions++
			}
			// Always expect a list pods action
			expectedNumberOfActions++

			if len(actions) != expectedNumberOfActions {
				t.Fatalf("Unexpected number of actions, got %v, want 3 (list pods, patch pod, delete pod)", len(actions))
			}

			var patchAction testclient.PatchAction
			var deleteAction testclient.DeleteAction

			for _, action := range actions {
				if action.GetVerb() == "patch" {
					patchAction = action.(testclient.PatchAction)
				}

				if action.GetVerb() == "delete" {
					deleteAction = action.(testclient.DeleteAction)
				}
			}

			if tc.expectedPatchedPod != nil {
				patchedPodBytes := patchAction.GetPatch()

				originalPod, err := json.Marshal(tc.pod)
				if err != nil {
					t.Fatalf("Failed to marshal original pod %#v: %v", originalPod, err)
				}
				updated, err := strategicpatch.StrategicMergePatch(originalPod, patchedPodBytes, v1.Pod{})
				if err != nil {
					t.Fatalf("Failed to apply strategic merge patch %q on pod %#v: %v", patchedPodBytes, originalPod, err)
				}

				updatedPod := &v1.Pod{}
				if err := json.Unmarshal(updated, updatedPod); err != nil {
					t.Fatalf("Failed to unmarshal updated pod %q: %v", updated, err)
				}

				if diff := cmp.Diff(tc.expectedPatchedPod, updatedPod, cmpopts.IgnoreFields(v1.Pod{}, "TypeMeta"), cmpopts.IgnoreFields(v1.PodCondition{}, "LastTransitionTime")); diff != "" {
					t.Fatalf("Unexpected diff on pod (-want,+got):\n%s", diff)
				}
			}

			if tc.expectedDeleteAction != nil {
				if diff := cmp.Diff(*tc.expectedDeleteAction, deleteAction, cmpopts.IgnoreFields(testclient.DeleteActionImpl{}, "ActionImpl")); diff != "" {
					t.Fatalf("Unexpected diff on deleteAction (-want,+got):\n%s", diff)
				}
			}
		})
	}
}
