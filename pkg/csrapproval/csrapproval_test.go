/*
Copyright 2019 The Kubernetes Authors.
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

package csrapproval

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"os"
	"testing"

	authorization "k8s.io/api/authorization/v1"
	capi "k8s.io/api/certificates/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	testclient "k8s.io/client-go/testing"
	"k8s.io/klog/v2"
)

func init() {
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
		}, nodeClientKeyUsages) != c.expected {
			t.Errorf("unexpected result of hasKubeletUsages(%v), expecting: %v", c.usages, c.expected)
		}
	}
}

type testValidator struct {
	Options
	t              *testing.T
	recognize      func(csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) bool
	validate       func(csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) (bool, error)
	preapprovehook func(csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) error
}

func (tv testValidator) Recognize(csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) bool {
	return tv.recognize(csr, x509cr)
}

func (tv testValidator) Validate(csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) (bool, error) {
	if tv.validate != nil {
		return tv.validate(csr, x509cr)
	}

	tv.t.Logf("testValidator: no validation function defined")
	return true, nil
}

func (tv testValidator) PreApproveHook(csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) error {
	if tv.preapprovehook != nil {
		return tv.preapprovehook(csr, x509cr)
	}
	return nil
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
		validate       func(csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) (bool, error)
		verifyActions  func(*testing.T, []testclient.Action)
		preApproveHook func(csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) error
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
			preApproveHook: func(_ *capi.CertificateSigningRequest, _ *x509.CertificateRequest) error {
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
			preApproveHook: func(_ *capi.CertificateSigningRequest, _ *x509.CertificateRequest) error {
				return fmt.Errorf("preApproveHook failed")
			},
			err: true,
		},
		{
			desc:       "recognized, allowed and validated",
			recognized: true,
			allowed:    true,
			validate: func(csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) (bool, error) {
				return true, nil
			},
			verifyActions: verifyCreateAndUpdate,
		},
		{
			desc:       "recognized, allowed but not validated",
			recognized: true,
			allowed:    true,
			validate: func(csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) (bool, error) {
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
			validate: func(csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) (bool, error) {
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

			validator := testValidator{
				Options: Options{
					Name:       "test validator",
					Label:      "testvalidator",
					ApproveMsg: "tester",
					Permission: authorization.ResourceAttributes{Group: "foo", Resource: "bar", Subresource: "baz"},
				},
				t: t,
				recognize: func(csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) bool {
					return c.recognized
				},
				validate:       c.validate,
				preapprovehook: c.preApproveHook,
			}
			approver := Context{
				Client: client,
				Vs:     []Validator{validator},
			}
			csr := makeTestCSR(t)
			if err := approver.HandleCSR(csr); err != nil && !c.err {
				t.Errorf("unexpected err: %v", err)
			}
			c.verifyActions(t, client.Actions())
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
		ObjectMeta: metav1.ObjectMeta{
			Name: "testcsr:" + b.cn,
		},
		Spec: capi.CertificateSigningRequestSpec{
			Username: b.requestor,
			Usages:   b.usages,
			Request:  bytes.TrimSpace(bytes.Join(blocks, nil)),
		},
	}
}
