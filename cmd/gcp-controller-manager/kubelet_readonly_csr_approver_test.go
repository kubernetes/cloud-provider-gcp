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
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	authorization "k8s.io/api/authorization/v1"
	capi "k8s.io/api/certificates/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	testclient "k8s.io/client-go/testing"
)

func TestKubeletReadonlyApprover(t *testing.T) {
	testCases := []struct {
		name                                                             string
		sarAllowed, podAnnotationAllowed, usageAllowed, autopilotEnabled bool
		expectCondition                                                  capi.CertificateSigningRequestCondition
	}{
		{
			name:                 "success when all valid",
			sarAllowed:           true,
			podAnnotationAllowed: true,
			usageAllowed:         true,
			autopilotEnabled:     true,
			expectCondition: capi.CertificateSigningRequestCondition{
				Type:    capi.CertificateApproved,
				Reason:  "AutoApproved",
				Message: "approved by kubelet readonly approver",
				Status:  v1.ConditionTrue,
			},
		},
		{
			name:                 "success when all valid but annotation bypassed",
			sarAllowed:           true,
			podAnnotationAllowed: false,
			usageAllowed:         true,
			autopilotEnabled:     false,
			expectCondition: capi.CertificateSigningRequestCondition{
				Type:    capi.CertificateApproved,
				Reason:  "AutoApproved",
				Message: "approved by kubelet readonly approver",
				Status:  v1.ConditionTrue,
			},
		},
		{
			name:                 "fail by all validation fail",
			sarAllowed:           false,
			podAnnotationAllowed: false,
			usageAllowed:         false,
			autopilotEnabled:     true,
			expectCondition: capi.CertificateSigningRequestCondition{
				Type:    capi.CertificateDenied,
				Reason:  "AutoDenied",
				Message: "csr usage any is not allowed, allowed usages: [\"key encipherment\",\"digital signature\",\"client auth\"]",
				Status:  v1.ConditionTrue,
			},
		},
		{
			name:                 "fail by sar validation fail",
			sarAllowed:           false,
			podAnnotationAllowed: true,
			usageAllowed:         true,
			autopilotEnabled:     true,
			expectCondition: capi.CertificateSigningRequestCondition{
				Type:    capi.CertificateDenied,
				Reason:  "AutoDenied",
				Message: "user test is not allowed to create a CSR",
				Status:  v1.ConditionTrue,
			},
		},
		{
			name:                 "fail by pod annotation validation fail",
			sarAllowed:           true,
			podAnnotationAllowed: false,
			usageAllowed:         true,
			autopilotEnabled:     true,
			expectCondition: capi.CertificateSigningRequestCondition{
				Type:    capi.CertificateDenied,
				Reason:  "AutoDenied",
				Message: "csr Extra is empty",
				Status:  v1.ConditionTrue,
			},
		},
		{
			name:                 "fail by usage validation fail",
			sarAllowed:           true,
			podAnnotationAllowed: true,
			usageAllowed:         false,
			autopilotEnabled:     true,
			expectCondition: capi.CertificateSigningRequestCondition{
				Type:    capi.CertificateDenied,
				Reason:  "AutoDenied",
				Message: "csr usage any is not allowed, allowed usages: [\"key encipherment\",\"digital signature\",\"client auth\"]",
				Status:  v1.ConditionTrue,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(fmt.Sprint(testCase.name), func(t *testing.T) {
			request := generateTestRequest(t, testCase.sarAllowed, testCase.podAnnotationAllowed, testCase.usageAllowed, testCase.autopilotEnabled)
			request.csr.Spec.SignerName = kubeletReadonlyCSRSignerName
			approver := newKubeletReadonlyCSRApprover(request.controllerContext)
			*autopilotEnabled = testCase.autopilotEnabled
			err := approver.handle(request.context, request.csr)
			if err != nil {
				t.Fatalf("error when handle approver, error: %v", err)
			}
			if diff := cmp.Diff(request.csr.Status.Conditions[0], testCase.expectCondition, cmp.Comparer(func(x, y capi.CertificateSigningRequestCondition) bool {
				return x.Type == y.Type && x.Reason == y.Reason && x.Message == y.Message && x.Status == y.Status
			})); diff != "" {
				t.Fatalf("condition don't match, diff -want +got\n%s", diff)
			}
		})
	}
}

func TestValidateCSRUsage(t *testing.T) {
	testUages := map[capi.KeyUsage]bool{
		capi.UsageSigning:           false,
		capi.UsageDigitalSignature:  true,
		capi.UsageContentCommitment: false,
		capi.UsageKeyEncipherment:   true,
		capi.UsageKeyAgreement:      false,
		capi.UsageDataEncipherment:  false,
		capi.UsageCertSign:          false,
		capi.UsageCRLSign:           false,
		capi.UsageEncipherOnly:      false,
		capi.UsageDecipherOnly:      false,
		capi.UsageAny:               false,
		capi.UsageServerAuth:        false,
		capi.UsageClientAuth:        true,
		capi.UsageCodeSigning:       false,
		capi.UsageEmailProtection:   false,
		capi.UsageSMIME:             false,
		capi.UsageIPsecEndSystem:    false,
		capi.UsageIPsecTunnel:       false,
		capi.UsageIPsecUser:         false,
		capi.UsageTimestamping:      false,
		capi.UsageOCSPSigning:       false,
		capi.UsageMicrosoftSGC:      false,
		capi.UsageNetscapeSGC:       false,
	}

	testCases := []struct {
		name         string
		request      kubeletReadonlyCSRRequest
		expectResult kubeletReadonlyCSRResponse
	}{
		{
			name: "fail by default",
			expectResult: kubeletReadonlyCSRResponse{
				result:  false,
				err:     nil,
				message: "csr usage is empty",
			},
		},
		{
			name: "all usage exist",
			request: generateTestRequestByUsage(t, []capi.KeyUsage{
				capi.UsageKeyEncipherment,
				capi.UsageDigitalSignature,
				capi.UsageClientAuth,
			}),
			expectResult: kubeletReadonlyCSRResponse{
				result:  true,
				err:     nil,
				message: "csr usage is valid",
			},
		},
		{
			name: "only key encipherment",
			request: generateTestRequestByUsage(t, []capi.KeyUsage{
				capi.UsageKeyEncipherment,
			}),
			expectResult: kubeletReadonlyCSRResponse{
				result:  true,
				err:     nil,
				message: "csr usage is valid",
			},
		},
	}

	for usage, allowed := range testUages {
		if allowed {
			testCases = append(testCases, struct {
				name         string
				request      kubeletReadonlyCSRRequest
				expectResult kubeletReadonlyCSRResponse
			}{
				name: fmt.Sprintf("csr usage %v is allowed", usage),
				request: generateTestRequestByUsage(t, []capi.KeyUsage{
					usage,
				}),
				expectResult: kubeletReadonlyCSRResponse{
					result:  true,
					err:     nil,
					message: "csr usage is valid",
				},
			})
		} else {
			testCases = append(testCases, struct {
				name         string
				request      kubeletReadonlyCSRRequest
				expectResult kubeletReadonlyCSRResponse
			}{
				name: fmt.Sprintf("csr usage %v is not allowed, allowed usages: [\"key encipherment\",\"digital signature\",\"client auth\"]", usage),
				request: generateTestRequestByUsage(t, []capi.KeyUsage{
					usage,
				}),
				expectResult: kubeletReadonlyCSRResponse{
					result:  false,
					err:     nil,
					message: fmt.Sprintf("csr usage %v is not allowed, allowed usages: [\"key encipherment\",\"digital signature\",\"client auth\"]", usage),
				},
			})
		}

	}

	for _, testCase := range testCases {
		t.Run(fmt.Sprint(testCase.name), func(t *testing.T) {
			response := validateCSRUsage(testCase.request)
			compareKubeletReadonlyCSRRequestResult(response, testCase.expectResult, t)
		})
	}
}

func TestValidatePodAnnotation(t *testing.T) {
	testCases := []struct {
		name             string
		annotation       string
		autopilotEnabled bool
		podName          string
		expectResult     kubeletReadonlyCSRResponse
	}{
		{
			name:             "fail by default",
			podName:          "test-default",
			autopilotEnabled: true,
			expectResult: kubeletReadonlyCSRResponse{
				result:  false,
				err:     nil,
				message: "pod test-default does not have annotation with key autopilot.gke.io/kubelet-api-limited-reader or the value is not \"true\"",
			},
		},
		{
			name:             "pod with annotation exists",
			annotation:       kubeletAPILimitedReaderAnnotationKey,
			podName:          "test-pod-annotation-exists",
			autopilotEnabled: true,
			expectResult: kubeletReadonlyCSRResponse{
				result:  true,
				err:     nil,
				message: fmt.Sprintf("Annotation validation passed."),
			},
		},
		{
			name:             "not auto pilot cluster",
			podName:          "test-default",
			autopilotEnabled: false,
			expectResult: kubeletReadonlyCSRResponse{
				result:  true,
				err:     nil,
				message: "Bypassing annotation validation because cluster is not a GKE Autopilot cluster.",
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(fmt.Sprint(testCase.name), func(t *testing.T) {
			request := generateTestRequestByPodName(t, testCase.podName)
			request.autopilotEnabled = testCase.autopilotEnabled
			if len(testCase.annotation) != 0 {
				createPodWithAnnotation(testCase.podName, testCase.annotation, *request.controllerContext)
			}
			response := validatePodAnnotation(request)
			compareKubeletReadonlyCSRRequestResult(response, testCase.expectResult, t)
		})
	}
}

func TestValidateRbac(t *testing.T) {
	testCases := []struct {
		name         string
		allowed      bool
		expectResult kubeletReadonlyCSRResponse
	}{
		{
			name:    "not allow to create csr",
			allowed: false,
			expectResult: kubeletReadonlyCSRResponse{
				result:  false,
				err:     nil,
				message: "user test is not allowed to create a CSR",
			},
		},
		{
			name:    "allow to create csr",
			allowed: true,
			expectResult: kubeletReadonlyCSRResponse{
				result:  true,
				err:     nil,
				message: "user test is allowed to create a CSR",
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(fmt.Sprint(testCase.name), func(t *testing.T) {
			request := generateTestRequestBySubjectAccessReview(t, testCase.allowed)
			request.csr.Spec.Username = "test"
			response := validateRbac(request)
			compareKubeletReadonlyCSRRequestResult(response, testCase.expectResult, t)
		})
	}
}

func compareKubeletReadonlyCSRRequestResult(response kubeletReadonlyCSRResponse, targetResult kubeletReadonlyCSRResponse, t *testing.T) {
	if diff := cmp.Diff(response, targetResult, cmp.Comparer(func(x, y kubeletReadonlyCSRResponse) bool {
		if x.err != nil {
			if y.err == nil || y.err.Error() != x.err.Error() {
				return false
			}
		}
		if y.err != nil {
			if x.err == nil || y.err.Error() != x.err.Error() {
				return false
			}
		}
		return x.result == y.result && x.message == y.message
	})); diff != "" {
		t.Fatalf("response don't match, diff -want +got\n%s", diff)
	}
}

func generateTestRequestByUsage(t *testing.T, usage []capi.KeyUsage) kubeletReadonlyCSRRequest {
	request := generateTestKubeletReadonlyCSRRequest(t)
	request.csr.Spec.Usages = usage
	return request
}

func createPodWithAnnotation(podName, annotation string, controllerContext controllerContext) {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      podName,
			Annotations: map[string]string{
				annotation: "true",
			},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "nginx",
					Image: "nginx:1.14.2",
					Ports: []v1.ContainerPort{
						{
							ContainerPort: 80,
						},
					},
				},
			},
		},
	}
	client := controllerContext.client.(*fake.Clientset)
	client.AddReactor("get", "pods", func(action testclient.Action) (handled bool, ret runtime.Object, err error) {
		return true, pod, nil
	})
}

func generateTestRequest(t *testing.T, sarAllowed, podAnnotationAllowed, usageAllowed, autopilotEnabled bool) kubeletReadonlyCSRRequest {
	request := generateTestRequestBySubjectAccessReview(t, sarAllowed)
	request.csr.Spec.Username = "test"
	request.csr.Spec.Extra = map[string]capi.ExtraValue{}
	request.autopilotEnabled = autopilotEnabled
	if podAnnotationAllowed {
		request.csr.Spec.Extra = map[string]capi.ExtraValue{
			podNameKey: []string{"test"},
		}
		createPodWithAnnotation("test", kubeletAPILimitedReaderAnnotationKey, *request.controllerContext)
	}
	if usageAllowed {
		request.csr.Spec.Usages = []capi.KeyUsage{
			capi.UsageKeyEncipherment,
			capi.UsageDigitalSignature,
			capi.UsageClientAuth,
		}
	} else {
		request.csr.Spec.Usages = []capi.KeyUsage{
			capi.UsageAny,
		}
	}
	return request
}

func generateTestRequestByPodName(t *testing.T, podName string) kubeletReadonlyCSRRequest {
	request := generateTestKubeletReadonlyCSRRequest(t)
	request.csr.Spec.Extra = map[string]capi.ExtraValue{
		podNameKey: []string{podName},
	}
	return request
}

func generateTestRequestBySubjectAccessReview(t *testing.T, allowed bool) kubeletReadonlyCSRRequest {
	request := generateTestKubeletReadonlyCSRRequest(t)
	client := request.controllerContext.client.(*fake.Clientset)
	client.AddReactor("create", "subjectaccessreviews", func(action testclient.Action) (handled bool, ret runtime.Object, err error) {
		return true, &authorization.SubjectAccessReview{
			Status: authorization.SubjectAccessReviewStatus{
				Allowed: allowed,
			},
		}, nil
	})
	return request
}

func generateTestKubeletReadonlyCSRRequest(t *testing.T) kubeletReadonlyCSRRequest {
	client := &fake.Clientset{}

	return kubeletReadonlyCSRRequest{
		context: context.TODO(),
		controllerContext: &controllerContext{
			client: client,
		},
		csr: makeTestCSR(t),
	}
}
