/*
Copyright 2022 The Kubernetes Authors.
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

	authorization "k8s.io/api/authorization/v1"
	capi "k8s.io/api/certificates/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cloud-provider-gcp/pkg/csrmetrics"
	"k8s.io/klog/v2"
	certutil "k8s.io/kubernetes/pkg/apis/certificates/v1"
	"k8s.io/kubernetes/pkg/controller/certificates"
)

const (
	kubeletReadonlyCSRSignerName         = "gke.io/kubelet-readonly-client"
	kubeletReadonlyCSRRequestMetrics     = "kubelet_readonly_csr_request"
	podNameKey                           = "authentication.kubernetes.io/pod-name"
	kubeletAPILimitedReaderAnnotationKey = "autopilot.gke.io/kubelet-api-limited-reader"
)

var (
	kubeletReadonlyUsageAllowList = []capi.KeyUsage{
		capi.UsageKeyEncipherment,
		capi.UsageDigitalSignature,
		capi.UsageClientAuth,
	}
)

type kubeletReadonlyCSRRequest struct {
	context           context.Context
	controllerContext *controllerContext
	csr               *capi.CertificateSigningRequest
	x509cr            *x509.CertificateRequest
	autopilotEnabled  bool
}

type kubeletReadonlyCSRResponse struct {
	result  bool
	err     error
	message string
}

// readonlyApprover handles approval/denial of CSRs access to read only port
// There is a list of approvers, requst must get all approval
type kubeletReadonlyCSRApprover struct {
	ctx        *controllerContext
	validators []kubeletReadonlyCsrValidator
}

func newKubeletReadonlyCSRApprover(ctx *controllerContext) kubeletReadonlyCSRApprover {
	return kubeletReadonlyCSRApprover{
		ctx:        ctx,
		validators: newKubeletReadonlyCsrValidator(),
	}
}

func (approver *kubeletReadonlyCSRApprover) handle(ctx context.Context, csr *capi.CertificateSigningRequest) error {
	if csr == nil || csr.Spec.SignerName != kubeletReadonlyCSRSignerName {
		return nil
	}
	klog.Infof("approver got CSR %q", csr.Name)
	recordMetric := csrmetrics.ApprovalStartRecorder(kubeletReadonlyCSRRequestMetrics)
	if len(csr.Status.Certificate) != 0 {
		return nil
	}
	if approved, denied := certificates.GetCertApprovalCondition(&csr.Status); approved || denied {
		return nil
	}

	x509cr, err := certutil.ParseCSR(csr.Spec.Request)
	if err != nil {
		recordMetric(csrmetrics.ApprovalStatusParseError)
		klog.Errorf("unable to parse csr %q: %v", csr.Name, err)
		return fmt.Errorf("unable to parse csr %q: %v", csr.Name, err)
	}

	for _, validator := range approver.validators {
		recordValidatorMetric := csrmetrics.ApprovalStartRecorder(validator.authFlowLabel)
		request := kubeletReadonlyCSRRequest{
			context:           ctx,
			controllerContext: approver.ctx,
			csr:               csr,
			x509cr:            x509cr,
			autopilotEnabled:  *autopilotEnabled,
		}
		response := validator.validate(request)

		switch {
		case response.err != nil:
			klog.Errorf("validating CSR %q: %v", csr.Name, err)
			return fmt.Errorf("validating CSR %q: %v", csr.Name, err)
		case response.result == false:
			klog.Infof("validator %q: denied CSR %q", validator.name, csr.Name)
			recordValidatorMetric(csrmetrics.ApprovalStatusDeny)
			return approver.updateCSR(csr, false, response.message)
		case response.result == true:
			klog.Infof("validator %q: approved CSR %q", validator.name, csr.Name)
			// Unique flow label will be logged
			recordValidatorMetric(csrmetrics.ApprovalStatusApprove)
		}
	}

	klog.Infof("all validators approved CSR %q", csr.Name)
	recordMetric(csrmetrics.ApprovalStatusApprove)
	return approver.updateCSR(csr, true, "approved by kubelet readonly approver")
}

func (approver *kubeletReadonlyCSRApprover) updateCSR(csr *capi.CertificateSigningRequest, approved bool, msg string) error {
	if approved {
		csr.Status.Conditions = append(csr.Status.Conditions, capi.CertificateSigningRequestCondition{
			Type:    capi.CertificateApproved,
			Reason:  "AutoApproved",
			Message: msg,
			Status:  v1.ConditionTrue,
		})
	} else {
		csr.Status.Conditions = append(csr.Status.Conditions, capi.CertificateSigningRequestCondition{
			Type:    capi.CertificateDenied,
			Reason:  "AutoDenied",
			Message: msg,
			Status:  v1.ConditionTrue,
		})
	}
	updateRecordMetric := csrmetrics.OutboundRPCStartRecorder("k8s.CertificateSigningRequests.updateApproval")
	_, err := approver.ctx.client.CertificatesV1().CertificateSigningRequests().UpdateApproval(context.TODO(), csr.Name, csr, metav1.UpdateOptions{})
	if err != nil {
		klog.Errorf("error updating approval status for csr: %v", err)
		updateRecordMetric(csrmetrics.OutboundRPCStatusError)
		return fmt.Errorf("error updating approval status for csr: %v", err)
	}
	updateRecordMetric(csrmetrics.OutboundRPCStatusOK)
	return nil
}

type kubeletReadonlyCsrValidator struct {
	name          string
	authFlowLabel string

	// validate will validate the request and return approved or denied.
	// (false, err): will base on err type retry or not. If it is a non-retryable error then deny the request.
	// (true, nil): will continue to next validater. If it is the last one, will approve the request.
	// (false, nil): reject the request.
	validate   func(request kubeletReadonlyCSRRequest) kubeletReadonlyCSRResponse
	permission authorization.ResourceAttributes
}

func newKubeletReadonlyCsrValidator() []kubeletReadonlyCsrValidator {
	return []kubeletReadonlyCsrValidator{
		{
			name:          "csr usage validator",
			authFlowLabel: "kubelet_readonly_csr_usage_match",
			validate:      validateCSRUsage,
		},
		{
			name:          "pod annotation validator",
			authFlowLabel: "kubelet_readonly_pod_annotation_valid",
			validate:      validatePodAnnotation,
		},
		{
			name:          "rbac validator",
			authFlowLabel: "kubelet_readonly_rbac_exist",
			validate:      validateRbac,
		},
	}
}

func validateCSRUsage(request kubeletReadonlyCSRRequest) kubeletReadonlyCSRResponse {
	csr := request.csr
	if csr == nil || len(csr.Spec.Usages) == 0 {
		klog.Errorf("csr usage is empty")
		return kubeletReadonlyCSRResponse{
			result:  false,
			err:     nil,
			message: "csr usage is empty",
		}
	}

	usageMap := map[capi.KeyUsage]struct{}{}
	for _, u := range kubeletReadonlyUsageAllowList {
		usageMap[u] = struct{}{}
	}

	for _, usage := range csr.Spec.Usages {
		if _, ok := usageMap[usage]; !ok {
			allowListString, err := json.Marshal(kubeletReadonlyUsageAllowList)
			if err != nil {
				klog.Errorf("error when marshal allowlist, error: %v", err)
				return kubeletReadonlyCSRResponse{
					result:  false,
					err:     err,
					message: fmt.Sprintf("error when marshal allowlist, error: %v", err),
				}
			}
			klog.Errorf("csr usage %v is not allowed, only allow any of %v", usage, string(allowListString))
			return kubeletReadonlyCSRResponse{
				result:  false,
				err:     nil,
				message: fmt.Sprintf("csr usage %v is not allowed, only allow any of %v", usage, string(allowListString)),
			}
		}
	}
	return kubeletReadonlyCSRResponse{
		result:  true,
		err:     nil,
		message: "csr usage is valid",
	}
}

func validatePodAnnotation(request kubeletReadonlyCSRRequest) kubeletReadonlyCSRResponse {
	if !request.autopilotEnabled {
		return kubeletReadonlyCSRResponse{
			result:  true,
			err:     nil,
			message: fmt.Sprintf("Cluster is not an GKE Autopilot cluster, Annotation validation bypass"),
		}
	}
	if request.csr == nil || len(request.csr.Spec.Extra) == 0 {
		klog.Errorf("csr Extra is empty")
		return kubeletReadonlyCSRResponse{
			result:  false,
			err:     nil,
			message: "csr Extra is empty",
		}
	}

	podNames, ok := request.csr.Spec.Extra[podNameKey]
	if !ok {
		klog.Errorf("csr does not have pod name attached")
		return kubeletReadonlyCSRResponse{
			result:  false,
			err:     fmt.Errorf("csr does not have pod name attached"),
			message: "csr does not have pod name attached",
		}
	}

	for _, podName := range podNames {
		pod, err := request.controllerContext.client.CoreV1().Pods("").Get(request.context, podName, metav1.GetOptions{})
		if err != nil {
			klog.Errorf("error when get pod %s, error: %v", podName, err)
			return kubeletReadonlyCSRResponse{
				result:  false,
				err:     fmt.Errorf("error when get pod %s, error: %v", podName, err),
				message: fmt.Sprintf("error when get pod %s, error: %v", podName, err),
			}
		}

		if pod.Annotations[kubeletAPILimitedReaderAnnotationKey] != "true" {
			klog.Errorf("Pod %s not have annotation %s or annotation is not \"true\" ", podName, kubeletAPILimitedReaderAnnotationKey)
			return kubeletReadonlyCSRResponse{
				result:  false,
				err:     nil,
				message: fmt.Sprintf("Pod %s not have annotation %s or annotation is not \"true\"", podName, kubeletAPILimitedReaderAnnotationKey),
			}
		}
		klog.Infof("Pod %s have annotation %s set to \"true\"", podName, kubeletAPILimitedReaderAnnotationKey)
	}

	return kubeletReadonlyCSRResponse{
		result:  true,
		err:     nil,
		message: fmt.Sprintf("Annotation validation pass"),
	}
}

// validateRbac include SAR validation
func validateRbac(request kubeletReadonlyCSRRequest) kubeletReadonlyCSRResponse {
	rattrs := authorization.ResourceAttributes{
		Group:    "certificates.k8s.io",
		Resource: "certificatesigningrequests/kubeletclient",
		Verb:     "create",
	}

	csr := request.csr
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
	sar, err := request.controllerContext.client.AuthorizationV1().SubjectAccessReviews().Create(request.context, sar, metav1.CreateOptions{})
	if err != nil {
		klog.Errorf("error when make subject access review, error: %v", err)
		return kubeletReadonlyCSRResponse{
			result:  false,
			err:     err,
			message: fmt.Sprintf("error when make subject access review, error: %v", err),
		}
	}

	if !sar.Status.Allowed {
		klog.Errorf("user %s is not allowed to create a CSR", csr.Spec.Username)
		return kubeletReadonlyCSRResponse{
			result:  false,
			err:     nil,
			message: fmt.Sprintf("user %s is not allowed to create a CSR", csr.Spec.Username),
		}
	}

	return kubeletReadonlyCSRResponse{
		result:  true,
		err:     nil,
		message: fmt.Sprintf("user %s is allowed to create a CSR", csr.Spec.Username),
	}
}
