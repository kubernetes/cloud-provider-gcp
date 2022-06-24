package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"testing"

	capi "k8s.io/api/certificates/v1"
	"k8s.io/client-go/kubernetes/fake"
	testclient "k8s.io/client-go/testing"
	"k8s.io/kubernetes/pkg/controller/certificates"
)

func TestIstiodApproverHandle(t *testing.T) {
	pk, err := ecdsa.GenerateKey(elliptic.P224(), insecureRand)
	if err != nil {
		t.Fatal(err)
	}

	tcs := []struct {
		desc          string
		csr           csrBuilder
		verifyActions func(*testing.T, []testclient.Action)
	}{
		{
			desc: "good",
			csr: csrBuilder{
				cn:         "system:serviceaccount:istio-system:istiod",
				requestor:  "system:serviceaccount:istio-system:istiod",
				signerName: istiodSignerName,
				usages: []capi.KeyUsage{
					capi.UsageKeyEncipherment,
					capi.UsageDigitalSignature,
					capi.UsageServerAuth,
				},
				dns: []string{
					"istiod.istio-system.svc",
					"istiod-remote.istio-system.svc",
					"istiod-123.istio-system.svc",
					"istio-pilot.istio-system.svc",
				},
				key: pk,
			},
			verifyActions: func(t *testing.T, as []testclient.Action) {
				if len(as) != 1 {
					t.Fatalf("expected 1 action, got: %d", len(as))
				}
				csr := as[0].(testclient.UpdateAction).GetObject().(*capi.CertificateSigningRequest)
				approved, _ := certificates.GetCertApprovalCondition(&csr.Status)
				if !approved {
					t.Fatalf("expected CSR to be approved: %#v", csr.Status)
				}
			},
		},
		{
			desc: "good",
			csr: csrBuilder{
				cn:         "system:serviceaccount:istio-system:istiod-123",
				requestor:  "system:serviceaccount:istio-system:istiod-123",
				signerName: istiodSignerName,
				usages: []capi.KeyUsage{
					capi.UsageKeyEncipherment,
					capi.UsageDigitalSignature,
					capi.UsageServerAuth,
				},
				dns: []string{"istiod-123.istio-system.svc"},
				key: pk,
			},
			verifyActions: func(t *testing.T, as []testclient.Action) {
				if len(as) != 1 {
					t.Fatalf("expected 1 action, got: %d", len(as))
				}
				csr := as[0].(testclient.UpdateAction).GetObject().(*capi.CertificateSigningRequest)
				approved, _ := certificates.GetCertApprovalCondition(&csr.Status)
				if !approved {
					t.Fatalf("expected CSR to be approved: %#v", csr.Status)
				}
			},
		},
		{
			desc: "ignore other signers",
			csr: csrBuilder{
				cn:         "system:serviceaccount:istio-system:istiod",
				requestor:  "system:serviceaccount:istio-system:istiod",
				signerName: "other",
				usages: []capi.KeyUsage{
					capi.UsageKeyEncipherment,
					capi.UsageDigitalSignature,
					capi.UsageServerAuth,
				},
				dns: []string{"istiod.istio-system.svc"},
				key: pk,
			},
			verifyActions: func(t *testing.T, as []testclient.Action) {
				if len(as) != 0 {
					t.Fatalf("expected 0 action, got: %d", len(as))
				}
			},
		},
		{
			desc: "cn doesn't match requester",
			csr: csrBuilder{
				cn:         "system:serviceaccount:istio-system:istioddd",
				requestor:  "system:serviceaccount:istio-system:istiod",
				signerName: istiodSignerName,
				usages: []capi.KeyUsage{
					capi.UsageKeyEncipherment,
					capi.UsageDigitalSignature,
					capi.UsageServerAuth,
				},
				dns: []string{"istiod.istio-system.svc"},
				key: pk,
			},
			verifyActions: func(t *testing.T, as []testclient.Action) {
				if len(as) != 1 {
					t.Fatalf("expected 1 action, got: %d", len(as))
				}
				csr := as[0].(testclient.UpdateAction).GetObject().(*capi.CertificateSigningRequest)
				_, denied := certificates.GetCertApprovalCondition(&csr.Status)
				if !denied {
					t.Fatalf("expected CSR to be denied: %#v", csr.Status)
				}
			},
		},
		{
			desc: "extra usage",
			csr: csrBuilder{
				cn:         "system:serviceaccount:istio-system:istiod",
				requestor:  "system:serviceaccount:istio-system:istiod",
				signerName: istiodSignerName,
				usages: []capi.KeyUsage{
					capi.UsageKeyEncipherment,
					capi.UsageDigitalSignature,
					capi.UsageServerAuth,
					capi.UsageClientAuth,
				},
				dns: []string{"istiod.istio-system.svc"},
				key: pk,
			},
			verifyActions: func(t *testing.T, as []testclient.Action) {
				if len(as) != 1 {
					t.Fatalf("expected 1 action, got: %d", len(as))
				}
				csr := as[0].(testclient.UpdateAction).GetObject().(*capi.CertificateSigningRequest)
				_, denied := certificates.GetCertApprovalCondition(&csr.Status)
				if !denied {
					t.Fatalf("expected CSR to be denied: %#v", csr.Status)
				}
			},
		},
		{
			desc: "extra dns",
			csr: csrBuilder{
				cn:         "system:serviceaccount:istio-system:istiod",
				requestor:  "system:serviceaccount:istio-system:istiod",
				signerName: istiodSignerName,
				usages: []capi.KeyUsage{
					capi.UsageKeyEncipherment,
					capi.UsageDigitalSignature,
					capi.UsageServerAuth,
				},
				dns: []string{"istiod.istio-system.svc", "other.istio-system.svc"},
				key: pk,
			},
			verifyActions: func(t *testing.T, as []testclient.Action) {
				if len(as) != 1 {
					t.Fatalf("expected 1 action, got: %d", len(as))
				}
				csr := as[0].(testclient.UpdateAction).GetObject().(*capi.CertificateSigningRequest)
				_, denied := certificates.GetCertApprovalCondition(&csr.Status)
				if !denied {
					t.Fatalf("expected CSR to be denied: %#v", csr.Status)
				}
			},
		},
		{
			desc: "extra san",
			csr: csrBuilder{
				cn:         "system:serviceaccount:istio-system:istiod",
				requestor:  "system:serviceaccount:istio-system:istiod",
				signerName: istiodSignerName,
				usages: []capi.KeyUsage{
					capi.UsageKeyEncipherment,
					capi.UsageDigitalSignature,
					capi.UsageServerAuth,
				},
				dns:    []string{"istiod.istio-system.svc"},
				emails: []string{"chewbaca@google.com"},
				key:    pk,
			},
			verifyActions: func(t *testing.T, as []testclient.Action) {
				if len(as) != 1 {
					t.Fatalf("expected 1 action, got: %d", len(as))
				}
				csr := as[0].(testclient.UpdateAction).GetObject().(*capi.CertificateSigningRequest)
				_, denied := certificates.GetCertApprovalCondition(&csr.Status)
				if !denied {
					t.Fatalf("expected CSR to be denied: %#v", csr.Status)
				}
			},
		},
		{
			desc: "wrong requestor",
			csr: csrBuilder{
				cn:         "system:serviceaccount:istio-system:istio-pilot",
				requestor:  "system:serviceaccount:istio-system:istio-pilot",
				signerName: istiodSignerName,
				usages: []capi.KeyUsage{
					capi.UsageKeyEncipherment,
					capi.UsageDigitalSignature,
					capi.UsageServerAuth,
				},
				dns: []string{"istiod.istio-system.svc"},
				key: pk,
			},
			verifyActions: func(t *testing.T, as []testclient.Action) {
				if len(as) != 1 {
					t.Fatalf("expected 1 action, got: %d", len(as))
				}
				csr := as[0].(testclient.UpdateAction).GetObject().(*capi.CertificateSigningRequest)
				_, denied := certificates.GetCertApprovalCondition(&csr.Status)
				if !denied {
					t.Fatalf("expected CSR to be denied: %#v", csr.Status)
				}
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.desc, func(t *testing.T) {
			client := &fake.Clientset{}
			approver := istiodApprover{
				ctx: &controllerContext{client: client},
			}

			csr := makeFancyTestCSR(t, tc.csr)
			if err := approver.handle(context.TODO(), csr); err != nil {
				t.Fatal(err)
			}
			tc.verifyActions(t, client.Actions())
		})
	}
}
