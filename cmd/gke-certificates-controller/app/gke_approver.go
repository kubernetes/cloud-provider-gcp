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
	"crypto/x509"
	"fmt"
	"reflect"
	"strings"

	authorization "k8s.io/api/authorization/v1beta1"
	capi "k8s.io/api/certificates/v1beta1"
	clientset "k8s.io/client-go/kubernetes"
	certutil "k8s.io/kubernetes/pkg/apis/certificates/v1beta1"
	"k8s.io/kubernetes/pkg/controller/certificates"
)

// gkeApprover handles approval/denial of CSRs based on SubjectAccessReview and
// CSR attestation data.
type gkeApprover struct {
	client     clientset.Interface
	validators []csrValidator
}

func newGKEApprover(client clientset.Interface) *gkeApprover {
	return &gkeApprover{client: client, validators: validators}
}

func (a *gkeApprover) handle(csr *capi.CertificateSigningRequest) error {
	if len(csr.Status.Certificate) != 0 {
		return nil
	}
	if approved, denied := certificates.GetCertApprovalCondition(&csr.Status); approved || denied {
		return nil
	}

	x509cr, err := certutil.ParseCSR(csr)
	if err != nil {
		return fmt.Errorf("unable to parse csr %q: %v", csr.Name, err)
	}

	var tried []string
	for _, r := range a.validators {
		if !r.recognize(csr, x509cr) {
			continue
		}
		tried = append(tried, r.permission.Subresource)
		if r.validate != nil && !r.validate(csr, x509cr) {
			return a.updateCSR(csr, false, r.denyMsg)
		}

		approved, err := a.authorizeSAR(csr, r.permission)
		if err != nil {
			return err
		}
		if approved {
			return a.updateCSR(csr, true, r.approveMsg)
		}
	}

	if len(tried) != 0 {
		return certificates.IgnorableError("recognized csr %q as %q but subject access review was not approved", csr.Name, tried)
	}
	return nil
}

func (a *gkeApprover) updateCSR(csr *capi.CertificateSigningRequest, approved bool, msg string) error {
	if approved {
		csr.Status.Conditions = append(csr.Status.Conditions, capi.CertificateSigningRequestCondition{
			Type:    capi.CertificateApproved,
			Reason:  "AutoApproved",
			Message: msg,
		})
	} else {
		csr.Status.Conditions = append(csr.Status.Conditions, capi.CertificateSigningRequestCondition{
			Type:    capi.CertificateDenied,
			Reason:  "AutoDenied",
			Message: msg,
		})
	}
	_, err := a.client.CertificatesV1beta1().CertificateSigningRequests().UpdateApproval(csr)
	if err != nil {
		return fmt.Errorf("error updating approval status for csr: %v", err)
	}
	return nil
}

type csrValidator struct {
	name       string
	approveMsg string
	denyMsg    string

	// recognize is a required field that returns true if this csrValidator is
	// applicable to given CSR.
	recognize func(csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) bool
	// validate is an optional field that returns true whether CSR should be
	// approved or denied.
	// If validate returns false, CSR is denied immediately.
	// If validate returns true, CSR proceeds to SubjectAccessReview check.
	validate func(csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) bool

	permission authorization.ResourceAttributes
}

// More specific validators go first.
var validators = []csrValidator{
	{
		name:       "self kubelet client certificate SubjectAccessReview",
		recognize:  isSelfNodeClientCert,
		permission: authorization.ResourceAttributes{Group: "certificates.k8s.io", Resource: "certificatesigningrequests", Verb: "create", Subresource: "selfnodeclient"},
		approveMsg: "Auto approving self kubelet client certificate after SubjectAccessReview.",
	},
	{
		name:       "kubelet client certificate SubjectAccessReview",
		recognize:  isNodeClientCert,
		permission: authorization.ResourceAttributes{Group: "certificates.k8s.io", Resource: "certificatesigningrequests", Verb: "create", Subresource: "nodeclient"},
		approveMsg: "Auto approving kubelet client certificate after SubjectAccessReview.",
	},
}

func (a *gkeApprover) authorizeSAR(csr *capi.CertificateSigningRequest, rattrs authorization.ResourceAttributes) (bool, error) {
	extra := make(map[string]authorization.ExtraValue)
	for k, v := range csr.Spec.Extra {
		extra[k] = authorization.ExtraValue(v)
	}

	sar := &authorization.SubjectAccessReview{
		Spec: authorization.SubjectAccessReviewSpec{
			User:               csr.Spec.Username,
			UID:                csr.Spec.UID,
			Groups:             csr.Spec.Groups,
			Extra:              extra,
			ResourceAttributes: &rattrs,
		},
	}
	sar, err := a.client.AuthorizationV1beta1().SubjectAccessReviews().Create(sar)
	if err != nil {
		return false, err
	}
	return sar.Status.Allowed, nil
}

func hasExactUsages(csr *capi.CertificateSigningRequest, usages []capi.KeyUsage) bool {
	if len(usages) != len(csr.Spec.Usages) {
		return false
	}

	usageMap := map[capi.KeyUsage]struct{}{}
	for _, u := range usages {
		usageMap[u] = struct{}{}
	}

	for _, u := range csr.Spec.Usages {
		if _, ok := usageMap[u]; !ok {
			return false
		}
	}

	return true
}

var kubeletClientUsages = []capi.KeyUsage{
	capi.UsageKeyEncipherment,
	capi.UsageDigitalSignature,
	capi.UsageClientAuth,
}

func isNodeClientCert(csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) bool {
	if !reflect.DeepEqual([]string{"system:nodes"}, x509cr.Subject.Organization) {
		return false
	}
	if (len(x509cr.DNSNames) > 0) || (len(x509cr.EmailAddresses) > 0) || (len(x509cr.IPAddresses) > 0) {
		return false
	}
	if !hasExactUsages(csr, kubeletClientUsages) {
		return false
	}
	if !strings.HasPrefix(x509cr.Subject.CommonName, "system:node:") {
		return false
	}
	return true
}

func isSelfNodeClientCert(csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) bool {
	if !isNodeClientCert(csr, x509cr) {
		return false
	}
	if csr.Spec.Username != x509cr.Subject.CommonName {
		return false
	}
	return true
}
