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

package app

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"k8s.io/klog"

	capi "k8s.io/api/certificates/v1beta1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/util/webhook"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	_ "k8s.io/kubernetes/pkg/apis/certificates/install" // Install certificates API group.
	"k8s.io/kubernetes/pkg/controller/certificates"
)

var (
	groupVersions = []schema.GroupVersion{capi.SchemeGroupVersion}

	csrSigningStatus = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "csr_signing_count",
		Help: "Count of signed CSRs",
	}, []string{"status"})
	csrSigningLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "csr_signing_latencies",
		Help: "Latency of CSR signer, in seconds",
	}, []string{"status"})
)

// ClusterSigningGKERetryBackoff is the backoff between GKE cluster signing retries.
const ClusterSigningGKERetryBackoff = 500 * time.Millisecond

func init() {
	prometheus.MustRegister(csrSigningStatus)
	prometheus.MustRegister(csrSigningLatency)
}

// gkeSigner uses external calls to GKE in order to sign certificate signing
// requests.
type gkeSigner struct {
	webhook        *webhook.GenericWebhook
	kubeConfigFile string
	retryBackoff   time.Duration
	recorder       record.EventRecorder
	client         clientset.Interface
}

// newGKESigner will create a new instance of a gkeSigner.
func newGKESigner(kubeConfigFile string, recorder record.EventRecorder, client clientset.Interface) (*gkeSigner, error) {
	webhook, err := webhook.NewGenericWebhook(legacyscheme.Scheme, legacyscheme.Codecs, kubeConfigFile, groupVersions, ClusterSigningGKERetryBackoff)
	if err != nil {
		return nil, err
	}

	return &gkeSigner{
		webhook:        webhook,
		kubeConfigFile: kubeConfigFile,
		retryBackoff:   ClusterSigningGKERetryBackoff,
		recorder:       recorder,
		client:         client,
	}, nil
}

func (s *gkeSigner) handle(csr *capi.CertificateSigningRequest) error {
	start := time.Now()
	if !certificates.IsCertificateRequestApproved(csr) {
		return nil
	}
	klog.Infof("gkeSigner triggered for %q", csr.Name)
	csr, err := s.sign(csr)
	if err != nil {
		csrSigningStatus.WithLabelValues("sign_error").Inc()
		csrSigningLatency.WithLabelValues("sign_error").Observe(time.Since(start).Seconds())
		return fmt.Errorf("error auto signing csr: %v", err)
	}
	_, err = s.client.CertificatesV1beta1().CertificateSigningRequests().UpdateStatus(csr)
	if err != nil {
		csrSigningStatus.WithLabelValues("update_error").Inc()
		csrSigningLatency.WithLabelValues("update_error").Observe(time.Since(start).Seconds())
		return fmt.Errorf("error updating signature for csr: %v", err)
	}
	klog.Infof("CSR %q signed", csr.Name)
	csrSigningStatus.WithLabelValues("signed").Inc()
	csrSigningLatency.WithLabelValues("signed").Observe(time.Since(start).Seconds())
	return nil
}

// Sign will make an external call to GKE order to sign the given
// *capi.CertificateSigningRequest, using the gkeSigner's
// kubeConfigFile.
func (s *gkeSigner) sign(csr *capi.CertificateSigningRequest) (*capi.CertificateSigningRequest, error) {
	result := s.webhook.WithExponentialBackoff(func() rest.Result {
		return s.webhook.RestClient.Post().Body(csr).Do()
	})

	if err := result.Error(); err != nil {
		if bodyErr := s.resultBodyError(result); bodyErr != nil {
			return nil, s.webhookError(csr, bodyErr)
		}
		return nil, s.webhookError(csr, err)
	}

	var statusCode int
	if result.StatusCode(&statusCode); statusCode < 200 || statusCode >= 300 {
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
	s.recorder.Eventf(csr, "Warning", "SigningError", "error while calling GKE: %v", err)
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
