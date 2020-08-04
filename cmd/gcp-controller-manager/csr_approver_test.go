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

	"github.com/google/go-tpm/tpm2"
	betacompute "google.golang.org/api/compute/v0.beta"
	compute "google.golang.org/api/compute/v1"
	container "google.golang.org/api/container/v1"
	authorization "k8s.io/api/authorization/v1beta1"
	capi "k8s.io/api/certificates/v1beta1"
	certsv1b1 "k8s.io/api/certificates/v1beta1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	testclient "k8s.io/client-go/testing"
	"k8s.io/cloud-provider-gcp/pkg/nodeidentity"
	"k8s.io/cloud-provider-gcp/pkg/tpmattest"
	"k8s.io/klog"
	certutil "k8s.io/kubernetes/pkg/apis/certificates/v1beta1"
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
			approver := gkeApprover{
				ctx:        &controllerContext{client: client},
				validators: []csrValidator{validator},
			}
			csr := makeTestCSR(t)
			if err := approver.handle(csr); err != nil && !c.err {
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
			b.signerName = stringPointer(certsv1b1.KubeAPIServerClientKubeletSignerName)
		}
		goodCases := []func(*csrBuilder, *controllerContext){goodCase}
		testRecognizer(t, "good", goodCases, isLegacyNodeClientCert, true)

		badCases := []func(*csrBuilder, *controllerContext){
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				b.signerName = nil // Should not recognize nil signer name
			},
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				b.signerName = stringPointer(certsv1b1.KubeletServingSignerName) // Should not recognize other signer name
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
			b.signerName = stringPointer(certsv1b1.KubeletServingSignerName)
		}
		goodCases := []func(*csrBuilder, *controllerContext){goodCase}
		testRecognizer(t, "good", goodCases, isNodeServerCert, true)

		badCases := []func(*csrBuilder, *controllerContext){
			func(b *csrBuilder, c *controllerContext) {},
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				b.signerName = nil // Should not recognize nil signer name
			},
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				b.signerName = stringPointer(certsv1b1.KubeAPIServerClientKubeletSignerName) // Should not recognize other signer name
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
		}
		cases := []func(*csrBuilder, *controllerContext){goodCase}
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
		}
		testValidator(t, "bad", cases, fn, false, false)
	})
	t.Run("isNodeClientCertWithAttestation", func(t *testing.T) {
		goodCase := func(b *csrBuilder, _ *controllerContext) {
			b.requestor = tpmKubeletUsername
			for _, name := range tpmAttestationBlocks {
				b.extraPEM[name] = []byte("foo")
			}
			b.signerName = stringPointer(certsv1b1.KubeAPIServerClientKubeletSignerName)
		}
		goodCases := []func(*csrBuilder, *controllerContext){goodCase}
		testRecognizer(t, "good", goodCases, isNodeClientCertWithAttestation, true)

		badCases := []func(*csrBuilder, *controllerContext){
			func(b *csrBuilder, c *controllerContext) {},
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				b.signerName = nil
			},
			func(b *csrBuilder, c *controllerContext) {
				goodCase(b, c)
				b.signerName = stringPointer(certsv1b1.KubeletServingSignerName)
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
				fakeCA.regenerateValidCert(t, nodeidentity.Identity{"z0", 1, "i9", 2, "p0"})
				defer fakeCA.regenerateValidCert(t, nodeidentity.Identity{"z0", 1, "i0", 2, "p0"})
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

			nodeID := nodeidentity.Identity{"z0", 1, "i0", 2, "p0"}
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
				nodeID := nodeidentity.Identity{"r0-a", 1, "i1", 2, "p0"}
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
				nodeID := nodeidentity.Identity{"z0", 1, "i9", 2, "p0"}
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
			csr := makeFancyTestCSR(b)
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
	return makeFancyTestCSR(csrBuilder{cn: "test-cert", key: pk})
}

type csrBuilder struct {
	cn         string
	orgs       []string
	requestor  string
	signerName *string
	usages     []capi.KeyUsage
	dns        []string
	emails     []string
	ips        []net.IP
	extraPEM   map[string][]byte
	key        *ecdsa.PrivateKey
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

	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		t.Logf("fakeGCPAPI request %q", req.URL.Path)
		switch req.URL.Path {
		case "/compute/v1/projects/p0/zones/z0/instances/i0":
			json.NewEncoder(rw).Encode(compute.Instance{
				Id:                1,
				Name:              "i0",
				Zone:              "z0",
				NetworkInterfaces: []*compute.NetworkInterface{{NetworkIP: "1.2.3.4"}},
			})
		case "/compute/v1/projects/p0/zones/z0/instances/i1":
			json.NewEncoder(rw).Encode(compute.Instance{
				Id:                2,
				Name:              "i1",
				Zone:              "z0",
				NetworkInterfaces: []*compute.NetworkInterface{{NetworkIP: "1.2.3.5"}},
			})
		case "/compute/v1/projects/2/zones/z0/instances/i0":
			json.NewEncoder(rw).Encode(compute.Instance{
				Id:                3,
				Name:              "i0",
				Zone:              "z0",
				NetworkInterfaces: []*compute.NetworkInterface{{NetworkIP: "1.2.3.4"}},
			})
		case "/compute/v1/projects/2/zones/r0-a/instances/i1":
			json.NewEncoder(rw).Encode(compute.Instance{
				Id:                4,
				Name:              "i0",
				Zone:              "r0-a",
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
		Type: tpm2.TagAttestCertify,
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
		node           *v1.Node
		instance       *compute.Instance
		shouldDelete   bool
		getInstanceErr error
		expectedErr    error
	}{
		{
			desc: "instance with 1 alias range and matches podCIDR",
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
			node: &v1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node-test",
				},
			},
		},
		{
			desc: "instance not found",
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
	}
	for _, c := range cases {
		fakeGetInstance := func(_ *controllerContext, _ string) (*compute.Instance, error) {
			return c.instance, c.getInstanceErr
		}
		shouldDelete, err := shouldDeleteNode(&controllerContext{}, c.node, fakeGetInstance)
		if err != c.expectedErr || shouldDelete != c.shouldDelete {
			t.Errorf("%s: shouldDeleteNode=(%v, %v), want (%v, %v)", c.desc, shouldDelete, err, c.shouldDelete, c.expectedErr)
		}
	}
}
