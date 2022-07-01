/*
Copyright 2017 The Kubernetes Authors.

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
	"context"
	"crypto/x509"
	"encoding/json"
	"fmt"

	capi "k8s.io/api/certificates/v1"
	certsv1 "k8s.io/api/certificates/v1"
	certsv1b1 "k8s.io/api/certificates/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	apiserveroptions "k8s.io/apiserver/pkg/server/options"
	"k8s.io/apiserver/pkg/util/webhook"
	"k8s.io/client-go/rest"
	"k8s.io/cloud-provider-gcp/pkg/csrmetrics"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	_ "k8s.io/kubernetes/pkg/apis/certificates/install" // Install certificates API group.
	certutil "k8s.io/kubernetes/pkg/apis/certificates/v1"
	"k8s.io/kubernetes/pkg/controller/certificates"
)

var (
	groupVersions = []schema.GroupVersion{capi.SchemeGroupVersion}

	// ClusterSigningGKERetryBackoff is the backoff between GKE cluster signing retries.
	ClusterSigningGKERetryBackoff *wait.Backoff = apiserveroptions.DefaultAuthWebhookRetryBackoff()
)

// gkeSigner uses external calls to GKE in order to sign certificate signing
// requests.
type gkeSigner struct {
	webhook      *webhook.GenericWebhook
	ctx          *controllerContext
	retryBackoff *wait.Backoff
	validators   []csrValidator
}

// newGKESigner will create a new instance of a gkeSigner.
func newGKESigner(ctx *controllerContext) (*gkeSigner, error) {
	clientConfig, err := webhook.LoadKubeconfig(ctx.clusterSigningGKEKubeconfig, nil)
	if err != nil {
		return nil, err
	}
	webhook, err := webhook.NewGenericWebhook(legacyscheme.Scheme, legacyscheme.Codecs, clientConfig, groupVersions, *ClusterSigningGKERetryBackoff)
	if err != nil {
		return nil, err
	}
	return &gkeSigner{
		webhook:      webhook,
		ctx:          ctx,
		retryBackoff: ClusterSigningGKERetryBackoff,
		validators:   csrValidators(ctx),
	}, nil
}

// handle is called by the generic Kubernetes certificate controller.
func (s *gkeSigner) handle(csr *capi.CertificateSigningRequest) error {
	_, _, err := s.handleInternal(csr)
	return err
}

func (s *gkeSigner) handleInternal(csr *capi.CertificateSigningRequest) (processed bool, out *capi.CertificateSigningRequest, err error) {
	if !certificates.IsCertificateRequestApproved(csr) {
		return false, nil, nil
	}

	// Ignore CSRs that are not addressed to the default signer.
	if csr.Spec.SignerName != "" &&
		csr.Spec.SignerName != certsv1.KubeAPIServerClientSignerName &&
		csr.Spec.SignerName != certsv1.KubeAPIServerClientKubeletSignerName &&
		csr.Spec.SignerName != certsv1.KubeletServingSignerName &&
		csr.Spec.SignerName != certsv1b1.LegacyUnknownSignerName &&
		csr.Spec.SignerName != istiodSignerName &&
		csr.Spec.SignerName != oidcSignerName {
		return false, nil, nil
	}

	klog.Infof("gkeSigner triggered for %q", csr.Name)

	recordMetric := csrmetrics.SigningStartRecorder(authFlowLabelNone)
	x509cr, err := certutil.ParseCSR(csr.Spec.Request)
	if err != nil {
		recordMetric(csrmetrics.SigningStatusParseError)
		return true, nil, fmt.Errorf("unable to parse csr %q: %v", csr.Name, err)
	}
	recordMetric = csrmetrics.SigningStartRecorder(s.metricLabel(csr, x509cr))
	csr, err = s.sign(csr)
	if err != nil {
		recordMetric(csrmetrics.SigningStatusSignError)
		return true, nil, fmt.Errorf("error auto signing csr: %v", err)
	}
	updateRecordMetric := csrmetrics.OutboundRPCStartRecorder("k8s.CertificateSigningRequests.updateStatus")
	csr, err = s.ctx.client.CertificatesV1().CertificateSigningRequests().UpdateStatus(context.TODO(), csr, metav1.UpdateOptions{})
	if err != nil {
		updateRecordMetric(csrmetrics.OutboundRPCStatusError)
		recordMetric(csrmetrics.SigningStatusUpdateError)
		return true, nil, fmt.Errorf("error updating signature for csr: %v", err)
	}

	updateRecordMetric(csrmetrics.OutboundRPCStatusOK)
	klog.Infof("CSR %q signed", csr.Name)
	recordMetric(csrmetrics.SigningStatusSigned)
	return true, csr, nil
}

func (s *gkeSigner) metricLabel(csr *capi.CertificateSigningRequest, x509cr *x509.CertificateRequest) string {
	for _, v := range s.validators {
		if v.recognize(csr, x509cr) {
			return v.authFlowLabel
		}
	}
	return authFlowLabelNone
}

// Sign will make an external call to GKE order to sign the given
// *capi.CertificateSigningRequest, using the gkeSigner's
// kubeConfigFile.
func (s *gkeSigner) sign(csr *capi.CertificateSigningRequest) (*capi.CertificateSigningRequest, error) {
	var statusCode int
	var result rest.Result
	webhook.WithExponentialBackoff(context.TODO(), *ClusterSigningGKERetryBackoff, func() error {
		recordMetric := csrmetrics.OutboundRPCStartRecorder("container.webhook.Sign")
		result = s.webhook.RestClient.Post().Body(csr).Do(context.TODO())
		if result.Error() != nil {
			recordMetric(csrmetrics.OutboundRPCStatusError)
			return result.Error()
		}
		if result.StatusCode(&statusCode); statusCode < 200 && statusCode >= 300 {
			recordMetric(csrmetrics.OutboundRPCStatusError)
		} else {
			recordMetric(csrmetrics.OutboundRPCStatusOK)
		}
		return nil
	}, webhook.DefaultShouldRetry)

	if err := result.Error(); err != nil {
		var webhookError error
		if bodyErr := s.resultBodyError(result); bodyErr != nil {
			webhookError = s.webhookError(csr, bodyErr)
		} else {
			webhookError = s.webhookError(csr, err)
		}
		return nil, webhookError
	}

	if statusCode < 200 || statusCode >= 300 {
		return nil, s.webhookError(csr, fmt.Errorf("received unsuccessful response code from webhook: %d", statusCode))
	}

	resultCSR := &capi.CertificateSigningRequest{}

	if err := result.Into(resultCSR); err != nil {
		return nil, s.webhookError(resultCSR, err)
	}

	// Keep the original CSR intact, and only update fields we expect to change.
	csr.Status.Certificate = resultCSR.Status.Certificate
	return csr, nil
}

func (s *gkeSigner) webhookError(csr *capi.CertificateSigningRequest, err error) error {
	klog.V(2).Infof("error contacting webhook backend: %s", err)
	s.ctx.recorder.Eventf(csr, "Warning", "SigningError", "error while calling GKE: %v", err)
	return err
}

// signResultError represents the structured response body of a failed call to
// GKE's SignCertificate API.
type signResultError struct {
	Error struct {
		Code    int
		Message string
		Status  string
	}
}

// resultBodyError attempts to extract an error out of a response body.
func (s *gkeSigner) resultBodyError(result rest.Result) error {
	body, _ := result.Raw()
	var sre signResultError
	if err := json.Unmarshal(body, &sre); err == nil {
		return fmt.Errorf("server responded with error: %s", sre.Error.Message)
	}
	return nil
}
