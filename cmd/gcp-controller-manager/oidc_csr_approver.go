package main

import (
	"context"
	"strings"

	capi "k8s.io/api/certificates/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	certutil "k8s.io/kubernetes/pkg/apis/certificates/v1"
	"k8s.io/kubernetes/pkg/controller/certificates"
)

const oidcSignerName = "pki.gke.io/gke-oidc"

func newOIDCApprover(ctx *controllerContext) *oidcApprover {
	return &oidcApprover{
		ctx: ctx,
	}
}

type oidcApprover struct {
	ctx *controllerContext
}

func (a *oidcApprover) handle(csr *capi.CertificateSigningRequest) error {
	if csr.Spec.SignerName != oidcSignerName {
		return nil
	}
	if approved, denied := certificates.GetCertApprovalCondition(&csr.Status); approved || denied {
		return nil
	}

	if !hasExactUsages(csr, []capi.KeyUsage{
		capi.UsageClientAuth,
		capi.UsageServerAuth,
	}) {
		return a.deny(csr, "disallowed usages requested")
	}

	x509cr, err := certutil.ParseCSR(csr.Spec.Request)
	if err != nil {
		return a.deny(csr, "unable to parse csr")
	}

	if len(x509cr.URIs) != 0 || len(x509cr.EmailAddresses) != 0 {
		return a.deny(csr, "disallowed sans requested")
	}

	if !strings.HasPrefix(csr.Spec.Username, "system:serviceaccount:anthos-identity-service:gke-oidc") {
		return a.deny(csr, "permission denied, disallowed requester")
	}

	if !a.validDomainNames(x509cr.DNSNames) {
		return a.deny(csr, "bad dns name")
	}

	return a.approve(csr)
}

func (a *oidcApprover) approve(csr *capi.CertificateSigningRequest) error {
	csr.Status.Conditions = append(csr.Status.Conditions, capi.CertificateSigningRequestCondition{
		Type:   capi.CertificateApproved,
		Reason: "AutoApproved",
		Status: v1.ConditionTrue,
	})
	_, err := a.ctx.client.CertificatesV1().CertificateSigningRequests().UpdateApproval(context.TODO(), csr.Name, csr, metav1.UpdateOptions{})
	return err
}

func (a *oidcApprover) deny(csr *capi.CertificateSigningRequest, msg string) error {
	csr.Status.Conditions = append(csr.Status.Conditions, capi.CertificateSigningRequestCondition{
		Type:    capi.CertificateDenied,
		Reason:  "AutoDenied",
		Message: msg,
		Status:  v1.ConditionTrue,
	})
	_, err := a.ctx.client.CertificatesV1().CertificateSigningRequests().UpdateApproval(context.TODO(), csr.Name, csr, metav1.UpdateOptions{})
	return err
}

func (a *oidcApprover) validDomainNames(names []string) bool {
	for _, name := range names {
		parts := strings.Split(name, ".")
		for _, part := range parts {
			if len(validation.NameIsDNS1035Label(part, false)) != 0 {
				return false
			}
		}
		if len(parts) != 3 {
			return false
		}
		if parts[2] != "svc" {
			return false
		}
		if parts[1] != "anthos-identity-service" && parts[1] != "kube-system" {
			return false
		}
		if parts[0] != "gke-oidc-envoy" &&
			!strings.HasPrefix(parts[0], "gke-oidc") {
			return false
		}
	}
	return true
}
