package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"net"
	"testing"

	capi "k8s.io/api/certificates/v1"
	"k8s.io/client-go/kubernetes/fake"
	testclient "k8s.io/client-go/testing"
	"k8s.io/kubernetes/pkg/controller/certificates"
)

func TestOIDCApproverHandle(t *testing.T) {
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
				cn:         "",
				requestor:  "system:serviceaccount:anthos-identity-service:gke-oidc-operator",
				signerName: oidcSignerName,
				usages: []capi.KeyUsage{
					capi.UsageClientAuth,
					capi.UsageServerAuth,
				},
				dns: []string{
					"gke-oidc-envoy.anthos-identity-service.svc",
				},
				ips: []net.IP{
					net.ParseIP("192.168.0.1"),
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
			desc: "ignore other signers",
			csr: csrBuilder{
				cn:         "",
				requestor:  "system:serviceaccount:anthos-identity-service:gke-oidc-operator",
				signerName: "other",
				usages: []capi.KeyUsage{
					capi.UsageClientAuth,
					capi.UsageServerAuth,
				},
				dns: []string{
					"gke-oidc-envoy.anthos-identity-service.svc",
				},
				ips: []net.IP{
					net.ParseIP("192.168.0.1"),
				},
				key: pk,
			},
			verifyActions: func(t *testing.T, as []testclient.Action) {
				if len(as) != 0 {
					t.Fatalf("expected 0 action, got: %d", len(as))
				}
			},
		},
		{
			desc: "extra usage",
			csr: csrBuilder{
				cn:         "",
				requestor:  "system:serviceaccount:anthos-identity-service:gke-oidc-operator",
				signerName: oidcSignerName,
				usages: []capi.KeyUsage{
					capi.UsageKeyEncipherment,
					capi.UsageDigitalSignature,
					capi.UsageServerAuth,
					capi.UsageClientAuth,
				},
				dns: []string{
					"gke-oidc-envoy.anthos-identity-service.svc",
				},
				ips: []net.IP{
					net.ParseIP("192.168.0.1"),
				},
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
				cn:         "",
				requestor:  "system:serviceaccount:anthos-identity-service:gke-oidc-operator",
				signerName: oidcSignerName,
				usages: []capi.KeyUsage{
					capi.UsageClientAuth,
					capi.UsageServerAuth,
				},
				dns: []string{"gke-oidc-envoy.anthos-identity-service.svc", "other.anthos-identity-service.svc"},
				ips: []net.IP{
					net.ParseIP("192.168.0.1"),
				},
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
				cn:         "",
				requestor:  "system:serviceaccount:anthos-identity-service:gke-oidc-operator",
				signerName: oidcSignerName,
				usages: []capi.KeyUsage{
					capi.UsageClientAuth,
					capi.UsageServerAuth,
				},
				dns: []string{
					"gke-oidc-envoy.anthos-identity-service.svc",
				},
				ips: []net.IP{
					net.ParseIP("192.168.0.1"),
				},
				emails: []string{"xyz@google.com"},
				key:    pk,
			},
			verifyActions: func(t *testing.T, as []testclient.Action) {
				if len(as) != 1 {
					t.Fatalf("expected 1 action, got: %d", len(as))
				}
				csr := as[0].(testclient.UpdateAction).GetObject().(*capi.CertificateSigningRequest)
				_, denied := certificates.GetCertApprovalCondition(&csr.Status)
				if !denied {
					t.Fatalf("expected CSR to be approved: %#v", csr.Status)
				}
			},
		},
		{
			desc: "wrong requestor",
			csr: csrBuilder{
				cn:         "",
				requestor:  "system:serviceaccount:anthos-identity-service:other",
				signerName: oidcSignerName,
				usages: []capi.KeyUsage{
					capi.UsageClientAuth,
					capi.UsageServerAuth,
				},
				dns: []string{
					"gke-oidc-envoy.anthos-identity-service.svc",
				},
				ips: []net.IP{
					net.ParseIP("192.168.0.1"),
				},
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
			approver := oidcApprover{
				ctx: &controllerContext{client: client},
			}

			csr := makeFancyTestCSR(t, tc.csr)
			if err := approver.handle(csr); err != nil {
				t.Fatal(err)
			}
			tc.verifyActions(t, client.Actions())
		})
	}
}
