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
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
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
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/go-tpm/tpm2"
	betacompute "google.golang.org/api/compute/v0.beta"
	compute "google.golang.org/api/compute/v1"
	container "google.golang.org/api/container/v1"
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
	"k8s.io/cloud-provider-gcp/pkg/nodeidentity"
	"k8s.io/cloud-provider-gcp/pkg/tpmattest"
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

// stringPointer copies a constant string and returns a pointer to the copy.
func stringPointer(str string) *string {
	return &str
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
	t.Run("isNodeClientCertWithAttestation", func(t *testing.T) {
		goodCase := func(b *csrBuilder, _ *controllerContext) {
			b.requestor = tpmKubeletUsername
			for _, name := range tpmAttestationBlocks {
				b.extraPEM[name] = []byte("foo")
			}
			b.signerName = certsv1.KubeAPIServerClientKubeletSignerName
		}
		goodCases := []func(*csrBuilder, *controllerContext){goodCase}
		testRecognizer(t, "good", goodCases, isNodeClientCertWithAttestation, true)

		badCases := []func(*csrBuilder, *controllerContext){
			func(b *csrBuilder, c *controllerContext) {},
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				b.signerName = ""
			},
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				b.signerName = certsv1.KubeletServingSignerName
			},
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				b.requestor = "awly"
			},
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				delete(b.extraPEM, tpmAttestationBlocks[1])
			},
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				b.cn = "awly"
			},
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				b.orgs = nil
			},
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				b.orgs = []string{"system:master"}
			},
		}
		testRecognizer(t, "bad", badCases, isNodeClientCertWithAttestation, false)
	})
	t.Run("validateTPMAttestation with cert", func(t *testing.T) {
		// TODO(awly): re-enable this when ATTESTATION CERTIFICATE is used.
		t.Skip()

		fakeCA, fakeCACache, cleanup := initFakeCACache(t)
		defer cleanup()
		client, srv := fakeGCPAPI(t, nil)
		defer srv.Close()

		goodCase := func(b *csrBuilder, c *controllerContext) {
			cs, err := compute.New(client)
			if err != nil {
				t.Fatalf("creating GCE API client: %v", err)
			}
			c.gcpCfg.Compute = cs

			c.gcpCfg.TPMEndorsementCACache = fakeCACache
			c.gcpCfg.ProjectID = "p0"
			b.requestor = tpmKubeletUsername
			b.cn = "system:node:i0"
			b.extraPEM["ATTESTATION CERTIFICATE"] = fakeCA.validCert.Raw

			attestData, attestSig := makeAttestationDataAndSignature(t, b.key, fakeCA.validCertKey)
			b.extraPEM["ATTESTATION DATA"] = attestData
			b.extraPEM["ATTESTATION SIGNATURE"] = attestSig
		}
		goodCases := []func(*csrBuilder, *controllerContext){goodCase}
		testValidator(t, "good", goodCases, validateTPMAttestation, true, false)

		badCases := []func(*csrBuilder, *controllerContext){
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				// Invalid CN format.
				b.cn = "awly"
			},
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				// CN valid but doesn't match name in ATTESTATION CERTIFICATE.
				b.cn = "system:node:i2"
			},
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				// Invalid AIK certificate
				b.extraPEM["ATTESTATION CERTIFICATE"] = []byte("invalid")
			},
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				// AIK certificate verification fails
				for _, cert := range fakeCA.invalidCerts {
					b.extraPEM["ATTESTATION CERTIFICATE"] = cert.Raw
					break
				}
			},
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				// ProjectID mismatch in nodeidentity
				c.gcpCfg.ProjectID = "p1"
			},
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				// Invalid attestation signature
				b.extraPEM["ATTESTATION SIGNATURE"] = []byte("invalid")
			},
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				// Attestation signature using wrong key
				key, err := rsa.GenerateKey(insecureRand, 2048)
				if err != nil {
					t.Fatal(err)
				}
				_, attestSig := makeAttestationDataAndSignature(t, b.key, key)
				b.extraPEM["ATTESTATION SIGNATURE"] = attestSig
			},
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				// Invalid attestation data
				b.extraPEM["ATTESTATION DATA"] = []byte("invalid")
			},
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				// Attestation data for wrong key
				key, err := ecdsa.GenerateKey(elliptic.P224(), insecureRand)
				if err != nil {
					t.Fatal(err)
				}
				attestData, _ := makeAttestationDataAndSignature(t, key, fakeCA.validCertKey)
				b.extraPEM["ATTESTATION DATA"] = attestData
			},
			func(b *csrBuilder, c *controllerContext) {
				// VM from nodeidentity doesn't exist
				fakeCA.regenerateValidCert(t, nodeidentity.Identity{Zone: "z0", ID: 1, Name: "i9", ProjectID: 2, ProjectName: "p0"})
				defer fakeCA.regenerateValidCert(t, nodeidentity.Identity{Zone: "z0", ID: 1, Name: "i0", ProjectID: 2, ProjectName: "p0"})
				goodCase(b, c)
			},

			// TODO: verifyclustermembership
		}
		testValidator(t, "bad", badCases, validateTPMAttestation, false, true)
	})
	t.Run("validateTPMAttestation with API", func(t *testing.T) {
		validKey, err := rsa.GenerateKey(insecureRand, 2048)
		if err != nil {
			t.Fatal(err)
		}
		gceClient, gceSrv := fakeGCPAPI(t, &validKey.PublicKey)
		defer gceSrv.Close()
		gkeClient, gkeSrv := fakeGKEAPI(t)
		defer gkeSrv.Close()

		goodCase := func(b *csrBuilder, c *controllerContext) {
			c.csrApproverVerifyClusterMembership = true

			cs, err := compute.New(gceClient)
			if err != nil {
				t.Fatalf("creating GCE API client: %v", err)
			}
			c.gcpCfg.Compute = cs
			bcs, err := betacompute.New(gceClient)
			if err != nil {
				t.Fatalf("creating GCE Beta API client: %v", err)
			}
			c.gcpCfg.BetaCompute = bcs
			ks, err := container.New(gkeClient)
			if err != nil {
				t.Fatalf("creating GKE API client: %v", err)
			}
			c.gcpCfg.Container = ks

			c.gcpCfg.ClusterName = "c0"
			c.gcpCfg.ProjectID = "p0"
			c.gcpCfg.Location = "z0"
			b.requestor = tpmKubeletUsername
			b.cn = "system:node:i0"

			nodeID := nodeidentity.Identity{Zone: "z0", ID: 1, Name: "i0", ProjectID: 2, ProjectName: "p0"}
			b.extraPEM["VM IDENTITY"], err = json.Marshal(nodeID)
			if err != nil {
				t.Fatalf("marshaling nodeID: %v", err)
			}

			attestData, attestSig := makeAttestationDataAndSignature(t, b.key, validKey)
			b.extraPEM["ATTESTATION DATA"] = attestData
			b.extraPEM["ATTESTATION SIGNATURE"] = attestSig
		}
		goodCases := []func(*csrBuilder, *controllerContext){
			goodCase,
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				// Instance is in a regional InstanceGroup, different zone from
				// cluster.
				b.cn = "system:node:i1"
				nodeID := nodeidentity.Identity{Zone: "r0-a", ID: 1, Name: "i1", ProjectID: 2, ProjectName: "p0"}
				b.extraPEM["VM IDENTITY"], err = json.Marshal(nodeID)
				if err != nil {
					t.Fatalf("marshaling nodeID: %v", err)
				}
			},
		}
		testValidator(t, "good", goodCases, validateTPMAttestation, true, false)

		badCases := []func(*csrBuilder, *controllerContext){
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				// Invalid CN format.
				b.cn = "awly"
			},
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				// CN valid but doesn't match name in ATTESTATION CERTIFICATE.
				b.cn = "system:node:i2"
			},
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				// ProjectID mismatch in nodeidentity
				c.gcpCfg.ProjectID = "p1"
			},
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				// Invalid attestation signature
				b.extraPEM["ATTESTATION SIGNATURE"] = []byte("invalid")
			},
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				// Attestation signature using wrong key
				key, err := rsa.GenerateKey(insecureRand, 2048)
				if err != nil {
					t.Fatal(err)
				}
				_, attestSig := makeAttestationDataAndSignature(t, b.key, key)
				b.extraPEM["ATTESTATION SIGNATURE"] = attestSig
			},
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				// Invalid attestation data
				b.extraPEM["ATTESTATION DATA"] = []byte("invalid")
			},
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				// Attestation data for wrong key
				key, err := ecdsa.GenerateKey(elliptic.P224(), insecureRand)
				if err != nil {
					t.Fatal(err)
				}
				attestData, _ := makeAttestationDataAndSignature(t, key, validKey)
				b.extraPEM["ATTESTATION DATA"] = attestData
			},
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				// VM doesn't belong to cluster NodePool.
				c.gcpCfg.ClusterName = "c1"
			},
		}
		testValidator(t, "bad", badCases, validateTPMAttestation, false, false)

		errorCases := []func(*csrBuilder, *controllerContext){
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				// Invalid VM identity
				b.extraPEM["VM IDENTITY"] = []byte("invalid")
			},
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				// VM from nodeidentity doesn't exist
				nodeID := nodeidentity.Identity{Zone: "z0", ID: 1, Name: "i9", ProjectID: 2, ProjectName: "p0"}
				b.extraPEM["VM IDENTITY"], err = json.Marshal(nodeID)
				if err != nil {
					t.Fatalf("marshaling nodeID: %v", err)
				}
			},
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				// Cluster fetch fails due to cluster name.
				c.gcpCfg.ClusterName = "unknown"
			},
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				// Cluster fetch fails due to cluster location.
				c.gcpCfg.Location = "unknown"
			},
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				// Cluster contains a non-existent NodePool.
				c.gcpCfg.ClusterName = "c2"
			},
		}
		testValidator(t, "error", errorCases, validateTPMAttestation, false, true)
	})

	t.Run("validateTPMAttestation with API ListReferrers", func(t *testing.T) {
		validKey, err := rsa.GenerateKey(insecureRand, 2048)
		if err != nil {
			t.Fatal(err)
		}
		gceClient, gceSrv := fakeGCPAPI(t, &validKey.PublicKey)
		defer gceSrv.Close()
		gkeClient, gkeSrv := fakeGKEAPI(t)
		defer gkeSrv.Close()

		goodCase := func(b *csrBuilder, c *controllerContext) {
			c.csrApproverVerifyClusterMembership = true
			c.csrApproverUseGCEInstanceListReferrers = true

			cs, err := compute.New(gceClient)
			if err != nil {
				t.Fatalf("creating GCE API client: %v", err)
			}
			c.gcpCfg.Compute = cs
			bcs, err := betacompute.New(gceClient)
			if err != nil {
				t.Fatalf("creating GCE Beta API client: %v", err)
			}
			c.gcpCfg.BetaCompute = bcs
			ks, err := container.New(gkeClient)
			if err != nil {
				t.Fatalf("creating GKE API client: %v", err)
			}
			c.gcpCfg.Container = ks

			c.gcpCfg.ClusterName = "c0"
			c.gcpCfg.ProjectID = "p0"
			c.gcpCfg.Location = "z0"
			b.requestor = tpmKubeletUsername
			b.cn = "system:node:i0"

			nodeID := nodeidentity.Identity{Zone: "z0", ID: 1, Name: "i0", ProjectID: 2, ProjectName: "p0"}
			b.extraPEM["VM IDENTITY"], err = json.Marshal(nodeID)
			if err != nil {
				t.Fatalf("marshaling nodeID: %v", err)
			}

			attestData, attestSig := makeAttestationDataAndSignature(t, b.key, validKey)
			b.extraPEM["ATTESTATION DATA"] = attestData
			b.extraPEM["ATTESTATION SIGNATURE"] = attestSig
		}
		goodCases := []func(*csrBuilder, *controllerContext){
			goodCase,
		}
		testValidator(t, "good", goodCases, validateTPMAttestation, true, false)

		badCases := []func(*csrBuilder, *controllerContext){
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				// Invalid CN format.
				b.cn = "awly"
			},
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				// CN valid but doesn't match name in ATTESTATION CERTIFICATE.
				b.cn = "system:node:i2"
			},
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				// ProjectID mismatch in nodeidentity
				c.gcpCfg.ProjectID = "p1"
			},
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				// Invalid attestation signature
				b.extraPEM["ATTESTATION SIGNATURE"] = []byte("invalid")
			},
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				// Attestation signature using wrong key
				key, err := rsa.GenerateKey(insecureRand, 2048)
				if err != nil {
					t.Fatal(err)
				}
				_, attestSig := makeAttestationDataAndSignature(t, b.key, key)
				b.extraPEM["ATTESTATION SIGNATURE"] = attestSig
			},
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				// Invalid attestation data
				b.extraPEM["ATTESTATION DATA"] = []byte("invalid")
			},
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				// Attestation data for wrong key
				key, err := ecdsa.GenerateKey(elliptic.P224(), insecureRand)
				if err != nil {
					t.Fatal(err)
				}
				attestData, _ := makeAttestationDataAndSignature(t, key, validKey)
				b.extraPEM["ATTESTATION DATA"] = attestData
			},
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				// VM doesn't belong to cluster NodePool.
				c.gcpCfg.ClusterName = "c1"
			},
		}
		testValidator(t, "bad", badCases, validateTPMAttestation, false, false)

		errorCases := []func(*csrBuilder, *controllerContext){
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				// Invalid VM identity
				b.extraPEM["VM IDENTITY"] = []byte("invalid")
			},
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				// VM from nodeidentity doesn't exist
				nodeID := nodeidentity.Identity{Zone: "z0", ID: 1, Name: "i9", ProjectID: 2, ProjectName: "p0"}
				b.extraPEM["VM IDENTITY"], err = json.Marshal(nodeID)
				if err != nil {
					t.Fatalf("marshaling nodeID: %v", err)
				}
			},
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				// Cluster fetch fails due to cluster name.
				c.gcpCfg.ClusterName = "unknown"
			},
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				// Cluster fetch fails due to cluster location.
				c.gcpCfg.Location = "unknown"
			},
		}
		testValidator(t, "error", errorCases, validateTPMAttestation, false, true)
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

func fakeGKEAPI(t *testing.T) (*http.Client, *httptest.Server) {
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		t.Logf("fakeGKEAPI request %q", req.URL.Path)
		switch req.URL.Path {
		case "/v1/projects/p0/locations/z0/clusters/c0":
			json.NewEncoder(rw).Encode(container.Cluster{
				Name: "c0",
				NodePools: []*container.NodePool{
					{InstanceGroupUrls: []string{"https://www.googleapis.com/compute/v1/projects/2/zones/r0/instanceGroupManagers/ig0"}},
					{InstanceGroupUrls: []string{"https://www.googleapis.com/compute/v1/projects/2/zones/z0/instanceGroupManagers/ig0"}},
				},
			})
		case "/v1/projects/p0/locations/z0/clusters/c1":
			json.NewEncoder(rw).Encode(container.Cluster{
				Name: "c1",
				NodePools: []*container.NodePool{
					{InstanceGroupUrls: []string{"https://www.googleapis.com/compute/v1/projects/2/zones/z0/instanceGroupManagers/ig1"}},
				},
			})
		case "/v1/projects/p0/locations/z0/clusters/c2":
			json.NewEncoder(rw).Encode(container.Cluster{
				Name: "c2",
				NodePools: []*container.NodePool{
					{InstanceGroupUrls: []string{"https://www.googleapis.com/compute/v1/projects/2/zones/z0/instanceGroupManagers/unknown"}},
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
	// Hash algorithm here must match tpmPub.NameAlg.
	tpmPubDigest := sha256.Sum256(tpmPubRaw)
	attestData, err := tpm2.AttestationData{
		// In go-tpm v0.3.0 validation was added to check that the magic value
		// is always 0xff544347
		// See https://github.com/google/go-tpm/pull/136 for details.
		Magic: 0xff544347,
		Type:  tpm2.TagAttestCertify,
		AttestedCertifyInfo: &tpm2.CertifyInfo{
			Name: tpm2.Name{
				Digest: &tpm2.HashValue{
					Alg:   tpm2.AlgSHA256,
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
			desc: "instance with 1 alias range and matches podCIDR",
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
								IpCidrRange: "10.0.0.1/24",
							},
						},
					},
				},
			},
		},
		{
			desc: "instance with 1 alias range doesn't match podCIDR",
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
						},
					},
				},
			},
			shouldDelete: true,
		},
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
		fakeGetInstance := func(_ *controllerContext, _ string) (*compute.Instance, error) {
			return c.instance, c.getInstanceErr
		}
		shouldDelete, err := shouldDeleteNode(c.ctx, c.node, fakeGetInstance)
		if err != c.expectedErr || shouldDelete != c.shouldDelete {
			t.Errorf("%s: shouldDeleteNode=(%v, %v), want (%v, %v)", c.desc, shouldDelete, err, c.shouldDelete, c.expectedErr)
		}
	}
}

func TestValidateInstanceGroupHint(t *testing.T) {
	cgr := []string{
		"https://www.googleapis.com/compute/v1/projects/1/zones/r0-a/instanceGroupManagers/rg1",
		"https://www.googleapis.com/compute/v1/projects/1/zones/r0/instanceGroupManagers/rg1",
		"https://www.googleapis.com/compute/v1/projects/2/zones/r0-a/instanceGroupManagers/rga2",
		"https://www.googleapis.com/compute/v1/projects/2/zones/z0/instanceGroupManagers/zg2",
	}
	for _, tc := range []struct {
		desc          string
		clusterGroups []string
		hintURL       string
		wantResolved  string
		wantError     bool
	}{
		{
			desc:          "hint has project number",
			hintURL:       "projects/1/zones/r0-a/instanceGroupManagers/rg1",
			clusterGroups: cgr,
			wantResolved:  cgr[0],
		}, {
			desc:          "hint has project id",
			hintURL:       "projects/p0/zones/r0-a/instanceGroupManagers/rg1",
			clusterGroups: cgr,
			wantResolved:  cgr[0],
		}, {
			desc:          "missing zone and ig name",
			hintURL:       "projects/p0",
			clusterGroups: cgr,
			wantResolved:  cgr[0],
			wantError:     true,
		}, {
			desc:          "hint not in clustergroups",
			hintURL:       "projects/p0/zones/r0-a/instanceGroupManagers/inv1",
			clusterGroups: cgr,
			wantError:     true,
		}, {
			desc:          "regional mig",
			hintURL:       "projects/4/zones/r0/instanceGroupManagers/rg1",
			clusterGroups: cgr,
			wantResolved:  cgr[1],
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			gotResolved, err := validateInstanceGroupHint(tc.clusterGroups, tc.hintURL)
			if (err != nil) != tc.wantError {
				t.Fatalf("unexpected error: got: %v, want: %t", err, tc.wantError)
				return
			}

			if err != nil {
				return
			}

			if gotResolved != tc.wantResolved {
				t.Fatalf("unexpected resolved url, got: %v, want: %v", gotResolved, tc.wantResolved)
			}
		})
	}
}

func TestCheckInstanceReferrers(t *testing.T) {
	for _, tc := range []struct {
		desc                     string
		clusterInstanceGroupUrls []string
		instance                 *compute.Instance
		gceClientHandler         func(rw http.ResponseWriter, req *http.Request)
		projectID                string
		wantOK                   bool
		wantErr                  bool
	}{
		{
			desc: "match found",
			clusterInstanceGroupUrls: []string{
				"https://www.googleapis.com/compute/v1/projects/p1/zones/z1/instanceGroupManagers/ig1",
			},
			instance: &compute.Instance{
				Name: "i1",
				Zone: "https://www.googleapis.com/compute/v1/projects/p1/zones/z1",
			},
			projectID: "z1",
			gceClientHandler: func(rw http.ResponseWriter, req *http.Request) {
				json.NewEncoder(rw).Encode(compute.InstanceListReferrers{
					Items: []*compute.Reference{
						{
							Referrer: "https://www.googleapis.com/compute/v1/projects/p1/zones/z1/instanceGroups/ig1",
						},
					},
				})
			},
			wantOK: true,
		},
		{
			desc: "match not found",
			clusterInstanceGroupUrls: []string{
				"https://www.googleapis.com/compute/v1/projects/p1/zones/z1/instanceGroupManagers/ig1",
			},
			instance: &compute.Instance{
				Name: "i1",
				Zone: "https://www.googleapis.com/compute/v1/projects/p1/zones/z1",
			},
			projectID: "z1",
			gceClientHandler: func(rw http.ResponseWriter, req *http.Request) {
				json.NewEncoder(rw).Encode(compute.InstanceListReferrers{
					Items: []*compute.Reference{
						{
							Referrer: "https://www.googleapis.com/compute/v1/projects/p1/zones/z1/instanceGroups/ig2",
						},
					},
				})
			},
			wantOK: false,
		},
		{
			desc: "error",
			clusterInstanceGroupUrls: []string{
				"https://www.googleapis.com/compute/v1/projects/p1/zones/z1/instanceGroupManagers/ig1",
			},
			instance: &compute.Instance{
				Name: "i1",
				Zone: "https://www.googleapis.com/compute/v1/projects/p1/zones/z1",
			},
			projectID: "z1",
			gceClientHandler: func(rw http.ResponseWriter, req *http.Request) {
				http.Error(rw, "not found", http.StatusNotFound)
			},
			wantErr: true,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(tc.gceClientHandler))
			defer srv.Close()
			cl := srv.Client()
			cl.Transport = fakeTransport{srv.URL}
			cs, err := compute.New(cl)
			if err != nil {
				t.Fatalf("failed to created compute service")
			}
			ctx := &controllerContext{
				gcpCfg: gcpConfig{
					ProjectID: tc.projectID,
					Compute:   cs,
				},
			}
			gotOK, err := checkInstanceReferrers(ctx, tc.instance, tc.clusterInstanceGroupUrls)
			if (err != nil) != tc.wantErr {
				t.Fatalf("got error: %v; want error: %v", err, tc.wantErr)
			}
			if gotOK != tc.wantOK {
				t.Errorf("got: %v; want: %v", gotOK, tc.wantOK)
			}
		})
	}
}

func TestParseInstanceGroupURL(t *testing.T) {
	for _, tc := range []struct {
		desc           string
		igURL          string
		wantIgLocation string
		wantIgName     string
		wantError      bool
	}{
		{
			desc:           "partial MIG url with project number",
			igURL:          "projects/2/zones/r0-a/instanceGroupManagers/ig1",
			wantIgLocation: "r0-a",
			wantIgName:     "ig1",
		},
		{
			desc:           "absolute zonal MIG url",
			igURL:          "https://www.googleapis.com/compute/v1/projects/2/zones/r0-a/instanceGroupManagers/ig1",
			wantIgLocation: "r0-a",
			wantIgName:     "ig1",
		},
		{
			desc:           "absolute regional MIG url",
			igURL:          "https://www.googleapis.com/compute/v1/projects/p0/regions/r0/instanceGroupManagers/ig2",
			wantIgLocation: "r0",
			wantIgName:     "ig2",
		},
		{
			desc:           "no protocol or hostname in MIG url",
			igURL:          "//compute/v1/projects/p0/regions/r0/instanceGroupManagers/ig2",
			wantIgLocation: "r0",
			wantIgName:     "ig2",
		},
		{
			desc:           "just MIG url path",
			igURL:          "/projects/p0/regions/r0/instanceGroupManagers/ig2",
			wantIgLocation: "r0",
			wantIgName:     "ig2",
		},
		{
			desc:           "zonal MIG url path",
			igURL:          "/projects/p0/zones/z0/instanceGroupManagers/ig3",
			wantIgLocation: "z0",
			wantIgName:     "ig3",
		},
		{
			desc:           "tail MIG url with location and name",
			igURL:          "zones/z0/instanceGroupManagers/ig3",
			wantIgLocation: "z0",
			wantIgName:     "ig3",
		},
		{
			desc:      "too few slashes",
			igURL:     "z0/instanceGroupManagers/ig3",
			wantError: true,
		},
		{
			desc:      "bare string with no slashes",
			igURL:     "z0",
			wantError: true,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			gotIgName, gotIgLocation, err := parseInstanceGroupURL(tc.igURL)
			if (err != nil) != tc.wantError {
				t.Fatalf("unexpected error: got: %v, want: %t", err, tc.wantError)
				return
			}

			if err != nil {
				return
			}

			if gotIgLocation != tc.wantIgLocation {
				t.Fatalf("unexpected igLocation, got: %v, want: %v", gotIgLocation, tc.wantIgLocation)
			}

			if gotIgName != tc.wantIgName {
				t.Fatalf("unexpected igName, got: %v, want: %v", gotIgName, tc.wantIgName)
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
							Type:    v1.DisruptionTarget,
							Status:  v1.ConditionTrue,
							Reason:  "DeletionByGCPControllerManager",
							Message: "GCPControllerManager: node no longer exists",
						},
						{
							Type:   v1.PodReady,
							Status: v1.ConditionTrue,
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
