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

// Package csrapproval handles validation for CSR approval requests.
package csrapproval

import (
	"context"
	"crypto/x509"
	"fmt"
	"reflect"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	authorization "k8s.io/api/authorization/v1beta1"
	capi "k8s.io/api/certificates/v1beta1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/cloud-provider-gcp/pkg/csrmetrics"
	"k8s.io/klog"
	certutil "k8s.io/kubernetes/pkg/apis/certificates/v1beta1"
)

var nodeClientKeyUsages = []capi.KeyUsage{
	capi.UsageKeyEncipherment,
	capi.UsageDigitalSignature,
	capi.UsageClientAuth,
}

var nodeServerKeyUsages = []capi.KeyUsage{
	capi.UsageKeyEncipherment,
	capi.UsageDigitalSignature,
	capi.UsageServerAuth,
}

// Validator represents a workflow to handle a CSR.
//
// HandleCSR processes certficate requests
// according to the decisions made with this interface.
// See below for details.
type Validator interface {
	// Return common parameters for this validator. See definition.
	Opts() Options

	// Should this request be handled by *this* Validator?. Others will be
	// attempted if you return false here.
	Recognize(csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) bool

	// If Recognize()'d, then validate the contents of the CSR.
	// For example, verify that the IP addresses or host names are
	// permitted by the requestor.
	Validate(csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) (bool, error)

	// Hook function that is called after Validate() is sucessful,
	// but before final approval. If this function returns an error,
	// this CSR will be retried.
	PreApproveHook(csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) error
}

// Options to be returned by validator.
// the Options() Validator function is implemented for you,
// so just embedding this struct ought to be sufficient
type Options struct {
	// Name for this validator, used in logging.
	Name string

	// Metrics Label
	Label string

	// Message to set when CSR is approved/denied
	ApproveMsg string
	DenyMsg    string

	// Subject Access Review Permissions
	Permission authorization.ResourceAttributes
}

// Opts returns itself. Validators that
// embed the Options struct don't need to
// implement the Validator.Options() separately.
func (o Options) Opts() Options {
	return o
}

// Context is the set of validators that are
// evaluated for each Certificate Signing Request.
type Context struct {
	// Set of Validators to be attempted.
	Vs []Validator

	// Kubernetes API client
	Client clientset.Interface
}

// Copied from k8s.io/kubernetes/pkg/controller/certificates/...
//
// This avoids the need for a much larger dependency tree from k8s.io/kubernetes

func getCertApprovalCondition(status *capi.CertificateSigningRequestStatus) (approved bool, denied bool) {
	for _, c := range status.Conditions {
		if c.Type == capi.CertificateApproved {
			approved = true
		}
		if c.Type == capi.CertificateDenied {
			denied = true
		}
	}
	return
}

// end copy from k8s.io/kubernetes...

// HandleCSR runs the certificate validation workflow.
//
// For each new CSR, HandleCSR will attempt to find a validator
// that can handle each CSR by calling v.Recognize(csr).
//
// If a validator is found, then the following checks are performed:
//
// - v.Validate(csr): Validate the SAN, IP address in the certificate.
//
// - SubjectAccessReview to ensure that the subject of the certificate
//	 has the Permission give in Options.Permission on the API server.
//
// - v.PreApproveHoook(csr) completes without error.
//
// If all of these are true, then the CSR is marked approved;
// or false otherwise.
//
// If there is an error at any step, this validation should be
// attempted again by calling HandleCSR(csr) later.
//
// If no Validator is Recognize()'d, this CSR is ignored.
func (vc *Context) HandleCSR(csr *capi.CertificateSigningRequest) error {
	recordMetric := csrmetrics.ApprovalStartRecorder("not_approved")
	if len(csr.Status.Certificate) != 0 {
		return nil
	}
	if approved, denied := getCertApprovalCondition(&csr.Status); approved || denied {
		return nil
	}
	klog.Infof("approver got CSR %q", csr.Name)

	x509cr, err := certutil.ParseCSR(csr.Spec.Request)
	if err != nil {
		recordMetric(csrmetrics.ApprovalStatusParseError)
		return fmt.Errorf("unable to parse csr %q: %v", csr.Name, err)
	}

	var tried []string
	for _, r := range vc.Vs {
		recordValidatorMetric := csrmetrics.ApprovalStartRecorder(r.Opts().Label)
		if !r.Recognize(csr, x509cr) {
			continue
		}

		klog.Infof("validator %q: matched CSR %q", r.Opts().Name, csr.Name)
		tried = append(tried, r.Opts().Name)

		ok, err := r.Validate(csr, x509cr)
		if err != nil {
			return fmt.Errorf("validating CSR %q: %v", csr.Name, err)
		}
		if !ok {
			klog.Infof("validator %q: denied CSR %q", r.Opts().Name, csr.Name)
			recordValidatorMetric(csrmetrics.ApprovalStatusDeny)
			return vc.updateCSR(csr, false, r.Opts().DenyMsg)
		}

		klog.Infof("CSR %q validation passed", csr.Name)

		approved, err := vc.subjectAccessReview(csr, r.Opts().Permission)
		if err != nil {
			recordValidatorMetric(csrmetrics.ApprovalStatusSARError)
			return err
		}

		if !approved {
			klog.Warningf("validator %q: SubjectAccessReview denied for CSR %q", r.Opts().Name, csr.Name)
			continue
		}
		klog.Infof("validator %q: SubjectAccessReview approved for CSR %q", r.Opts().Name, csr.Name)

		if err := r.PreApproveHook(csr, x509cr); err != nil {
			klog.Warningf("validator %q: preApproveHook failed for CSR %q: %v", r.Opts().Name, csr.Name, err)
			recordValidatorMetric(csrmetrics.ApprovalStatusPreApproveHookError)
			return err
		}

		klog.Infof("validator %q: preApproveHook passed for CSR %q", r.Opts().Name, csr.Name)
		recordValidatorMetric(csrmetrics.ApprovalStatusApprove)
		return vc.updateCSR(csr, true, r.Opts().ApproveMsg)
	}

	if len(tried) != 0 {
		recordMetric(csrmetrics.ApprovalStatusSARReject)
		return fmt.Errorf("recognized csr %q as %q but subject access review was not approved", csr.Name, tried)
	}

	klog.Infof("no validators matched CSR %q", csr.Name)
	recordMetric(csrmetrics.ApprovalStatusIgnore)
	return nil
}

func (vc *Context) updateCSR(csr *capi.CertificateSigningRequest, approved bool, msg string) error {
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
	_, err := vc.Client.CertificatesV1beta1().CertificateSigningRequests().UpdateApproval(context.TODO(), csr, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("error updating approval status for csr: %v", err)
	}
	return nil
}

func (vc *Context) subjectAccessReview(csr *capi.CertificateSigningRequest, rattrs authorization.ResourceAttributes) (bool, error) {
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
	sar, err := vc.Client.AuthorizationV1beta1().SubjectAccessReviews().Create(context.TODO(), sar, metav1.CreateOptions{})
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

func isNodeCert(x509cr *x509.CertificateRequest) bool {
	if !reflect.DeepEqual([]string{"system:nodes"}, x509cr.Subject.Organization) {
		return false
	}

	if len(x509cr.EmailAddresses) > 0 {
		return false
	}

	return strings.HasPrefix(x509cr.Subject.CommonName, "system:node:")
}

// IsNodeClientCert recognizes client certificates
func IsNodeClientCert(csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) bool {
	if !isNodeCert(x509cr) {
		return false
	}

	if len(x509cr.DNSNames) > 0 || len(x509cr.IPAddresses) > 0 {
		return false
	}

	return hasExactUsages(csr, nodeClientKeyUsages)
}

// IsNodeServerCert recognizes server certificates
func IsNodeServerCert(csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) bool {
	if !isNodeCert(x509cr) {
		return false
	}

	if !hasExactUsages(csr, nodeServerKeyUsages) {
		return false
	}

	return csr.Spec.Username == x509cr.Subject.CommonName
}
