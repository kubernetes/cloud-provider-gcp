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
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
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
	"testing"

	"github.com/google/go-tpm/tpm2"
	betacompute "google.golang.org/api/compute/v0.beta"
	compute "google.golang.org/api/compute/v1"
	authorization "k8s.io/api/authorization/v1beta1"
	capi "k8s.io/api/certificates/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	testclient "k8s.io/client-go/testing"
	"k8s.io/cloud-provider-gcp/pkg/nodeidentity"
	"k8s.io/cloud-provider-gcp/pkg/tpmattest"
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
		validate      validateFunc
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
			validate: func(opts GCPConfig, csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) (bool, error) {
				return true, nil
			},
			verifyActions: verifyCreateAndUpdate,
		},
		{
			desc:       "recognized, allowed but not validated",
			recognized: true,
			allowed:    true,
			validate: func(opts GCPConfig, csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) (bool, error) {
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
		{
			desc:       "recognized, allowed but validation failed",
			recognized: true,
			allowed:    true,
			validate: func(opts GCPConfig, csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) (bool, error) {
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
			csr := makeTestCSR(t)
			if err := approver.handle(csr); err != nil && !c.err {
				t.Errorf("unexpected err: %v", err)
			}
			c.verifyActions(t, client.Actions())
		})
	}
}

func TestValidators(t *testing.T) {
	t.Run("isLegacyNodeClientCert", func(t *testing.T) {
		goodCase := func(b *csrBuilder, _ *GCPConfig) { b.requestor = legacyKubeletUsername }
		goodCases := []func(*csrBuilder, *GCPConfig){goodCase}
		testValidator(t, "good", goodCases, isLegacyNodeClientCert, true)

		badCases := []func(*csrBuilder, *GCPConfig){
			func(b *csrBuilder, c *GCPConfig) {
				goodCase(b, c)
				b.cn = "mike"
			},
			func(b *csrBuilder, _ *GCPConfig) {},
			func(b *csrBuilder, c *GCPConfig) {
				goodCase(b, c)
				b.orgs = nil
			},
			func(b *csrBuilder, c *GCPConfig) {
				goodCase(b, c)
				b.orgs = []string{"system:master"}
			},
			func(b *csrBuilder, c *GCPConfig) {
				goodCase(b, c)
				b.usages = kubeletServerUsages
			},
		}
		testValidator(t, "bad", badCases, isLegacyNodeClientCert, false)
	})
	t.Run("isNodeServerClient", func(t *testing.T) {
		goodCase := func(b *csrBuilder, _ *GCPConfig) { b.usages = kubeletServerUsages }
		goodCases := []func(*csrBuilder, *GCPConfig){goodCase}
		testValidator(t, "good", goodCases, isNodeServerCert, true)

		badCases := []func(*csrBuilder, *GCPConfig){
			func(b *csrBuilder, c *GCPConfig) {},
			func(b *csrBuilder, c *GCPConfig) {
				goodCase(b, c)
				b.cn = "mike"
			},
			func(b *csrBuilder, c *GCPConfig) {
				goodCase(b, c)
				b.orgs = nil
			},
			func(b *csrBuilder, c *GCPConfig) {
				goodCase(b, c)
				b.orgs = []string{"system:master"}
			},
			func(b *csrBuilder, c *GCPConfig) {
				goodCase(b, c)
				b.requestor = "joe"
			},
			func(b *csrBuilder, c *GCPConfig) {
				goodCase(b, c)
				b.cn = "system:node:bar"
			},
		}
		testValidator(t, "bad", badCases, isNodeServerCert, false)
	})
	t.Run("validateNodeServerCertInner", func(t *testing.T) {
		client, srv := fakeGCPAPI(t, nil)
		defer srv.Close()
		fn := func(opts GCPConfig, csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) bool {
			cs, err := compute.New(client)
			if err != nil {
				t.Fatalf("creating GCE API client: %v", err)
			}
			opts.Compute = cs
			ok, err := validateNodeServerCert(opts, csr, x509cr)
			if err != nil {
				t.Fatalf("validateNodeServerCert: %v", err)
			}
			return ok
		}

		goodCase := func(b *csrBuilder, c *GCPConfig) {
			c.ProjectID = "p0"
			c.Zones = []string{"z1", "z0"}
			b.requestor = "system:node:i0"
			b.ips = []net.IP{net.ParseIP("1.2.3.4")}
		}
		cases := []func(*csrBuilder, *GCPConfig){goodCase}
		testValidator(t, "good", cases, fn, true)

		cases = []func(*csrBuilder, *GCPConfig){
			func(b *csrBuilder, c *GCPConfig) {},
			// No Name.
			func(b *csrBuilder, c *GCPConfig) {
				goodCase(b, c)
				b.requestor = ""
			},
			// No IPAddresses.
			func(b *csrBuilder, c *GCPConfig) {
				goodCase(b, c)
				b.ips = nil
			},
			// Wrong project.
			func(b *csrBuilder, c *GCPConfig) {
				goodCase(b, c)
				c.ProjectID = "p99"
			},
			// Wrong zone.
			func(b *csrBuilder, c *GCPConfig) {
				goodCase(b, c)
				c.Zones = []string{"z99"}
			},
			// Wrong instance name.
			func(b *csrBuilder, c *GCPConfig) {
				goodCase(b, c)
				b.requestor = "i99"
			},
			// Not matching IP.
			func(b *csrBuilder, c *GCPConfig) {
				goodCase(b, c)
				b.ips = []net.IP{net.ParseIP("1.2.3.5")}
			},
		}
		testValidator(t, "bad", cases, fn, false)
	})
	t.Run("isNodeClientCertWithAttestation", func(t *testing.T) {
		goodCase := func(b *csrBuilder, _ *GCPConfig) {
			b.requestor = tpmKubeletUsername
			for _, name := range tpmAttestationBlocks {
				b.extraPEM[name] = []byte("foo")
			}
		}
		goodCases := []func(*csrBuilder, *GCPConfig){goodCase}
		testValidator(t, "good", goodCases, isNodeClientCertWithAttestation, true)

		badCases := []func(*csrBuilder, *GCPConfig){
			func(b *csrBuilder, c *GCPConfig) {},
			func(b *csrBuilder, c *GCPConfig) {
				goodCase(b, c)
				b.requestor = "awly"
			},
			func(b *csrBuilder, c *GCPConfig) {
				goodCase(b, c)
				delete(b.extraPEM, tpmAttestationBlocks[1])
			},
			func(b *csrBuilder, c *GCPConfig) {
				goodCase(b, c)
				b.cn = "awly"
			},
			func(b *csrBuilder, c *GCPConfig) {
				goodCase(b, c)
				b.orgs = nil
			},
			func(b *csrBuilder, c *GCPConfig) {
				goodCase(b, c)
				b.orgs = []string{"system:master"}
			},
		}
		testValidator(t, "bad", badCases, isNodeClientCertWithAttestation, false)
	})
	t.Run("validateTPMAttestation with cert", func(t *testing.T) {
		// TODO(awly): re-enable this when ATTESTATION CERTIFICATE is used.
		t.Skip()

		fakeCA, fakeCACache, cleanup := initFakeCACache(t)
		defer cleanup()
		client, srv := fakeGCPAPI(t, nil)
		defer srv.Close()

		validateFunc := func(opts GCPConfig, csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) bool {
			ok, err := validateTPMAttestation(opts, csr, x509cr)
			return ok && err == nil
		}

		goodCase := func(b *csrBuilder, c *GCPConfig) {
			cs, err := compute.New(client)
			if err != nil {
				t.Fatalf("creating GCE API client: %v", err)
			}
			c.Compute = cs

			c.TPMEndorsementCACache = fakeCACache
			c.ProjectID = "p0"
			b.requestor = tpmKubeletUsername
			b.cn = "system:node:i0"
			b.extraPEM["ATTESTATION CERTIFICATE"] = fakeCA.validCert.Raw

			attestData, attestSig := makeAttestationDataAndSignature(t, b.key, fakeCA.validCertKey)
			b.extraPEM["ATTESTATION DATA"] = attestData
			b.extraPEM["ATTESTATION SIGNATURE"] = attestSig
		}
		goodCases := []func(*csrBuilder, *GCPConfig){goodCase}
		testValidator(t, "good", goodCases, validateFunc, true)

		badCases := []func(*csrBuilder, *GCPConfig){
			func(b *csrBuilder, c *GCPConfig) {
				goodCase(b, c)
				// Invalid CN format.
				b.cn = "awly"
			},
			func(b *csrBuilder, c *GCPConfig) {
				goodCase(b, c)
				// CN valid but doesn't match name in ATTESTATION CERTIFICATE.
				b.cn = "system:node:i2"
			},
			func(b *csrBuilder, c *GCPConfig) {
				goodCase(b, c)
				// Invalid AIK certificate
				b.extraPEM["ATTESTATION CERTIFICATE"] = []byte("invalid")
			},
			func(b *csrBuilder, c *GCPConfig) {
				goodCase(b, c)
				// AIK certificate verification fails
				for _, cert := range fakeCA.invalidCerts {
					b.extraPEM["ATTESTATION CERTIFICATE"] = cert.Raw
					break
				}
			},
			func(b *csrBuilder, c *GCPConfig) {
				goodCase(b, c)
				// ProjectID mismatch in nodeidentity
				c.ProjectID = "p1"
			},
			func(b *csrBuilder, c *GCPConfig) {
				goodCase(b, c)
				// Invalid attestation signature
				b.extraPEM["ATTESTATION SIGNATURE"] = []byte("invalid")
			},
			func(b *csrBuilder, c *GCPConfig) {
				goodCase(b, c)
				// Attestation signature using wrong key
				key, err := rsa.GenerateKey(insecureRand, 2048)
				if err != nil {
					t.Fatal(err)
				}
				_, attestSig := makeAttestationDataAndSignature(t, b.key, key)
				b.extraPEM["ATTESTATION SIGNATURE"] = attestSig
			},
			func(b *csrBuilder, c *GCPConfig) {
				goodCase(b, c)
				// Invalid attestation data
				b.extraPEM["ATTESTATION DATA"] = []byte("invalid")
			},
			func(b *csrBuilder, c *GCPConfig) {
				goodCase(b, c)
				// Attestation data for wrong key
				key, err := ecdsa.GenerateKey(elliptic.P224(), insecureRand)
				if err != nil {
					t.Fatal(err)
				}
				attestData, _ := makeAttestationDataAndSignature(t, key, fakeCA.validCertKey)
				b.extraPEM["ATTESTATION DATA"] = attestData
			},
			func(b *csrBuilder, c *GCPConfig) {
				// VM from nodeidentity doesn't exist
				fakeCA.regenerateValidCert(t, nodeidentity.Identity{"z0", 1, "i9", 2, "p0"})
				defer fakeCA.regenerateValidCert(t, nodeidentity.Identity{"z0", 1, "i0", 2, "p0"})
				goodCase(b, c)
			},

			// TODO: verifyclustermembership
		}
		testValidator(t, "bad", badCases, validateFunc, false)
	})
	t.Run("validateTPMAttestation with API", func(t *testing.T) {
		validKey, err := rsa.GenerateKey(insecureRand, 2048)
		if err != nil {
			t.Fatal(err)
		}
		client, srv := fakeGCPAPI(t, &validKey.PublicKey)
		defer srv.Close()

		validateFunc := func(opts GCPConfig, csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) bool {
			ok, err := validateTPMAttestation(opts, csr, x509cr)
			return ok && err == nil
		}

		goodCase := func(b *csrBuilder, c *GCPConfig) {
			cs, err := compute.New(client)
			if err != nil {
				t.Fatalf("creating GCE API client: %v", err)
			}
			c.Compute = cs
			bcs, err := betacompute.New(client)
			if err != nil {
				t.Fatalf("creating GCE Beta API client: %v", err)
			}
			c.BetaCompute = bcs

			c.ProjectID = "p0"
			b.requestor = tpmKubeletUsername
			b.cn = "system:node:i0"

			nodeID := nodeidentity.Identity{"z0", 1, "i0", 2, "p0"}
			b.extraPEM["VM IDENTITY"], err = json.Marshal(nodeID)
			if err != nil {
				t.Fatalf("marshaling nodeID: %v", err)
			}

			attestData, attestSig := makeAttestationDataAndSignature(t, b.key, validKey)
			b.extraPEM["ATTESTATION DATA"] = attestData
			b.extraPEM["ATTESTATION SIGNATURE"] = attestSig
		}
		goodCases := []func(*csrBuilder, *GCPConfig){goodCase}
		testValidator(t, "good", goodCases, validateFunc, true)

		badCases := []func(*csrBuilder, *GCPConfig){
			func(b *csrBuilder, c *GCPConfig) {
				goodCase(b, c)
				// Invalid CN format.
				b.cn = "awly"
			},
			func(b *csrBuilder, c *GCPConfig) {
				goodCase(b, c)
				// CN valid but doesn't match name in ATTESTATION CERTIFICATE.
				b.cn = "system:node:i2"
			},
			func(b *csrBuilder, c *GCPConfig) {
				goodCase(b, c)
				// Invalid VM identity
				b.extraPEM["VM IDENTITY"] = []byte("invalid")
			},
			func(b *csrBuilder, c *GCPConfig) {
				goodCase(b, c)
				// ProjectID mismatch in nodeidentity
				c.ProjectID = "p1"
			},
			func(b *csrBuilder, c *GCPConfig) {
				goodCase(b, c)
				// Invalid attestation signature
				b.extraPEM["ATTESTATION SIGNATURE"] = []byte("invalid")
			},
			func(b *csrBuilder, c *GCPConfig) {
				goodCase(b, c)
				// Attestation signature using wrong key
				key, err := rsa.GenerateKey(insecureRand, 2048)
				if err != nil {
					t.Fatal(err)
				}
				_, attestSig := makeAttestationDataAndSignature(t, b.key, key)
				b.extraPEM["ATTESTATION SIGNATURE"] = attestSig
			},
			func(b *csrBuilder, c *GCPConfig) {
				goodCase(b, c)
				// Invalid attestation data
				b.extraPEM["ATTESTATION DATA"] = []byte("invalid")
			},
			func(b *csrBuilder, c *GCPConfig) {
				goodCase(b, c)
				// Attestation data for wrong key
				key, err := ecdsa.GenerateKey(elliptic.P224(), insecureRand)
				if err != nil {
					t.Fatal(err)
				}
				attestData, _ := makeAttestationDataAndSignature(t, key, validKey)
				b.extraPEM["ATTESTATION DATA"] = attestData
			},
			func(b *csrBuilder, c *GCPConfig) {
				goodCase(b, c)
				// VM from nodeidentity doesn't exist
				nodeID := nodeidentity.Identity{"z0", 1, "i9", 2, "p0"}
				b.extraPEM["VM IDENTITY"], err = json.Marshal(nodeID)
				if err != nil {
					t.Fatalf("marshaling nodeID: %v", err)
				}
			},

			// TODO: verifyclustermembership
		}
		testValidator(t, "bad", badCases, validateFunc, false)
	})
}

func testValidator(t *testing.T, desc string, cases []func(b *csrBuilder, c *GCPConfig), checkFunc recognizeFunc, want bool) {
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

func makeTestCSR(t *testing.T) *capi.CertificateSigningRequest {
	pk, err := ecdsa.GenerateKey(elliptic.P224(), insecureRand)
	if err != nil {
		t.Fatal(err)
	}
	return makeFancyTestCSR(csrBuilder{cn: "test-cert", key: pk})
}

type csrBuilder struct {
	cn        string
	orgs      []string
	requestor string
	usages    []capi.KeyUsage
	dns       []string
	emails    []string
	ips       []net.IP
	extraPEM  map[string][]byte
	key       *ecdsa.PrivateKey
}

func makeFancyTestCSR(b csrBuilder) *capi.CertificateSigningRequest {
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
		panic(err)
	}
	blocks := [][]byte{pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrb})}
	for typ, data := range b.extraPEM {
		blocks = append(blocks, pem.EncodeToMemory(&pem.Block{Type: typ, Bytes: data}))
	}
	return &capi.CertificateSigningRequest{
		Spec: capi.CertificateSigningRequestSpec{
			Username: b.requestor,
			Usages:   b.usages,
			Request:  bytes.TrimSpace(bytes.Join(blocks, nil)),
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
		case "/compute/v1/projects/2/zones/z0/instances/i0":
			json.NewEncoder(rw).Encode(compute.Instance{
				Name:              "i0",
				NetworkInterfaces: []*compute.NetworkInterface{{NetworkIP: "1.2.3.4"}},
			})
		case "/compute/beta/projects/2/zones/z0/instances/i0/getShieldedVmIdentity":
			json.NewEncoder(rw).Encode(betacompute.ShieldedVmIdentity{
				SigningKey: &betacompute.ShieldedVmIdentityEntry{
					EkPub: string(ekPubPEM),
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

func makeAttestationDataAndSignature(t *testing.T, csrKey *ecdsa.PrivateKey, aikKey *rsa.PrivateKey) ([]byte, []byte) {
	tpmPub, err := tpmattest.MakePublic(csrKey.Public())
	if err != nil {
		t.Fatal(err)
	}
	tpmPubRaw, err := tpmPub.Encode()
	if err != nil {
		t.Fatal(err)
	}
	tpmPubDigest := sha1.Sum(tpmPubRaw)
	attestData, err := tpm2.AttestationData{
		Type: tpm2.TagAttestCertify,
		AttestedCertifyInfo: &tpm2.CertifyInfo{
			Name: tpm2.Name{
				Digest: &tpm2.HashValue{
					Alg:   tpm2.AlgSHA1,
					Value: tpmPubDigest[:],
				},
			},
		},
	}.Encode()
	if err != nil {
		t.Fatal(err)
	}

	attestDataDigest := sha256.Sum256(attestData)
	sig, err := aikKey.Sign(insecureRand, attestDataDigest[:], crypto.SHA256)
	if err != nil {
		t.Fatal(err)
	}

	return attestData, sig
}
