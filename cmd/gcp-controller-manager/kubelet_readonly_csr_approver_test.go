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
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/rand"
	"testing"

	"github.com/google/go-cmp/cmp"
	authorization "k8s.io/api/authorization/v1"
	capi "k8s.io/api/certificates/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	testclient "k8s.io/client-go/testing"
)

func TestKubeletReadonlyApprover(t *testing.T) {
	testCases := []struct {
		name                                                             string
		sarAllowed, podAnnotationAllowed, usageAllowed, autopilotEnabled bool
		sarWantError, podAnnotationWantError                             bool
		commonName                                                       string
		expectCondition                                                  capi.CertificateSigningRequestCondition
		expectError                                                      error
	}{
		{
			name:                 "success when all valid",
			sarAllowed:           true,
			podAnnotationAllowed: true,
			usageAllowed:         true,
			autopilotEnabled:     true,
			commonName:           kubeletReadonlyCRCommonName,
			expectCondition: capi.CertificateSigningRequestCondition{
				Type:    capi.CertificateApproved,
				Reason:  "AutoApproved",
				Message: "approved by kubelet readonly approver",
				Status:  v1.ConditionTrue,
			},
		},
		{
			name:                 fmt.Sprintf("fail when common name is not %s", kubeletReadonlyCRCommonName),
			sarAllowed:           true,
			podAnnotationAllowed: true,
			usageAllowed:         true,
			autopilotEnabled:     true,
			commonName:           "test fail",
			expectCondition: capi.CertificateSigningRequestCondition{
				Type:    capi.CertificateDenied,
				Reason:  "AutoDenied",
				Message: "x509 common name should start with kubelet-ro-client:",
				Status:  v1.ConditionTrue,
			},
		},
		{
			name:         "success when all valid but annotation bypassed",
			sarAllowed:   true,
			usageAllowed: true,
			commonName:   kubeletReadonlyCRCommonName,
			expectCondition: capi.CertificateSigningRequestCondition{
				Type:    capi.CertificateApproved,
				Reason:  "AutoApproved",
				Message: "approved by kubelet readonly approver",
				Status:  v1.ConditionTrue,
			},
		},
		{
			name:             "fail by all validation fail",
			autopilotEnabled: true,
			commonName:       "test fail",
			expectCondition: capi.CertificateSigningRequestCondition{
				Type:    capi.CertificateDenied,
				Reason:  "AutoDenied",
				Message: "csr usage any is not allowed, allowed usages: [\"key encipherment\",\"digital signature\",\"client auth\"]",
				Status:  v1.ConditionTrue,
			},
		},
		{
			name:                 "fail by sar validation fail",
			podAnnotationAllowed: true,
			usageAllowed:         true,
			autopilotEnabled:     true,
			commonName:           kubeletReadonlyCRCommonName,
			expectCondition: capi.CertificateSigningRequestCondition{
				Type:    capi.CertificateDenied,
				Reason:  "AutoDenied",
				Message: "user system:serviceaccount:default:test is not allowed to create a CSR",
				Status:  v1.ConditionTrue,
			},
		},
		{
			name:             "fail by pod annotation validation fail",
			sarAllowed:       true,
			usageAllowed:     true,
			autopilotEnabled: true,
			commonName:       kubeletReadonlyCRCommonName,
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
			autopilotEnabled:     true,
			commonName:           kubeletReadonlyCRCommonName,
			expectCondition: capi.CertificateSigningRequestCondition{
				Type:    capi.CertificateDenied,
				Reason:  "AutoDenied",
				Message: "csr usage any is not allowed, allowed usages: [\"key encipherment\",\"digital signature\",\"client auth\"]",
				Status:  v1.ConditionTrue,
			},
		},
		{
			name:                 "fail by sar validation error",
			sarAllowed:           true,
			podAnnotationAllowed: true,
			usageAllowed:         true,
			autopilotEnabled:     true,
			sarWantError:         true,
			commonName:           kubeletReadonlyCRCommonName,
			expectError:          fmt.Errorf("validating CSR \"fail by sar validation error\" failed: validator \"rbac validator\" has error failed to get subject access review"),
		},
		{
			name:                   "fail by pod annotation validation error",
			sarAllowed:             true,
			podAnnotationAllowed:   true,
			usageAllowed:           true,
			autopilotEnabled:       true,
			podAnnotationWantError: true,
			commonName:             kubeletReadonlyCRCommonName,
			expectError:            fmt.Errorf("validating CSR \"fail by pod annotation validation error\" failed: validator \"pod annotation validator\" has error failed to get pods, error: failed to get pod"),
		},
	}

	for _, testCase := range testCases {
		t.Run(fmt.Sprint(testCase.name), func(t *testing.T) {
			request := generateTestRequest(t, &kubeletReadonlyTestRequestBuilder{
				sarAllowed:             testCase.sarAllowed,
				podAnnotationAllowed:   testCase.podAnnotationAllowed,
				usageAllowed:           testCase.usageAllowed,
				autopilotEnabled:       testCase.autopilotEnabled,
				podAnnotationWantError: testCase.podAnnotationWantError,
				sarWantError:           testCase.sarWantError,
				commonName:             testCase.commonName,
			})
			request.csr.Name = testCase.name
			request.csr.Spec.SignerName = kubeletReadonlyCSRSignerName
			approver := newKubeletReadonlyCSRApprover(request.controllerContext)
			*autopilotEnabled = testCase.autopilotEnabled
			err := approver.handle(request.context, request.csr)
			switch {
			case testCase.expectError == nil && err == nil:
				if diff := cmp.Diff(request.csr.Status.Conditions[0], testCase.expectCondition, cmp.Comparer(func(x, y capi.CertificateSigningRequestCondition) bool {
					return x.Type == y.Type && x.Reason == y.Reason && x.Message == y.Message && x.Status == y.Status
				})); diff != "" {
					t.Fatalf("condition don't match, diff -want +got\n%s", diff)
				}
			case testCase.expectError != nil && err == nil ||
				err != nil && testCase.expectError == nil ||
				err.Error() != testCase.expectError.Error():
				t.Fatalf("error not match, got: %v, expect: %v", err, testCase.expectError)
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
		wantError    bool
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
				wantError    bool
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
				wantError    bool
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
		podNames         []string
		podUIDs          []string
		namespace        string
		username         string
		wantError        bool
		expectResult     kubeletReadonlyCSRResponse
	}{
		{
			name:             "fail by default",
			podNames:         []string{"test-default"},
			podUIDs:          []string{"test-default-uid"},
			namespace:        "default",
			username:         "system:serviceaccount:default:test-default",
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
			podNames:         []string{"test-pod-annotation-exists"},
			podUIDs:          []string{"test-pod-annotation-exists-uid"},
			namespace:        "default",
			username:         "system:serviceaccount:default:test-pod-annotation-exists",
			autopilotEnabled: true,
			expectResult: kubeletReadonlyCSRResponse{
				result:  true,
				err:     nil,
				message: fmt.Sprintf("Annotation validation passed."),
			},
		},
		{
			name:      "not auto pilot cluster",
			podNames:  []string{"test-autopilot-not-enabled"},
			podUIDs:   []string{"test-autopilot-not-enabled-uid"},
			namespace: "default",
			username:  "system:serviceaccount:default:test-autopilot-not-enabled",
			expectResult: kubeletReadonlyCSRResponse{
				result:  true,
				err:     nil,
				message: "Bypassing annotation validation because cluster is not a GKE Autopilot cluster.",
			},
		},
		{
			name:             "error when get pod",
			podNames:         []string{"test-get-pod-fail"},
			podUIDs:          []string{"test-get-pod-fail-uid"},
			namespace:        "default",
			username:         "system:serviceaccount:default:test-get-pod-fail",
			wantError:        true,
			annotation:       kubeletAPILimitedReaderAnnotationKey,
			autopilotEnabled: true,
			expectResult: kubeletReadonlyCSRResponse{
				err:     fmt.Errorf("failed to get pods, error: failed to get pod"),
				message: "failed to get pods, error: failed to get pod",
			},
		},
		{
			name:             "missing pod name",
			podUIDs:          []string{"test-missing-pod-name-UID"},
			namespace:        "default",
			username:         "system:serviceaccount:default:test-missing-pod-name-UID",
			wantError:        true,
			annotation:       kubeletAPILimitedReaderAnnotationKey,
			autopilotEnabled: true,
			expectResult: kubeletReadonlyCSRResponse{
				message: "csr does not have pod name attached",
			},
		},
		{
			name:             "missing pod uid",
			podNames:         []string{"test-missing-pod-uid"},
			namespace:        "default",
			username:         "system:serviceaccount:default:test-missing-pod-uid",
			wantError:        true,
			annotation:       kubeletAPILimitedReaderAnnotationKey,
			autopilotEnabled: true,
			expectResult: kubeletReadonlyCSRResponse{
				message: "csr does not have pod UID attached",
			},
		},
		{
			name:             "username invalid",
			podNames:         []string{"test-username-invalid"},
			podUIDs:          []string{"test-username-invalid-UID"},
			namespace:        "default",
			username:         "default",
			wantError:        true,
			annotation:       kubeletAPILimitedReaderAnnotationKey,
			autopilotEnabled: true,
			expectResult: kubeletReadonlyCSRResponse{
				err:     fmt.Errorf("Username must be in the form system:serviceaccount:namespace:name"),
				message: "failed to get namespace from username default, err: Username must be in the form system:serviceaccount:namespace:name",
			},
		},
		{
			name:             "length of pod name array and UID array do not match",
			podNames:         []string{"test-len-not-match", "test-len-not-match2"},
			podUIDs:          []string{"test-len-not-match-UID"},
			namespace:        "default",
			username:         "system:serviceaccount:default:test-len-not-match",
			wantError:        true,
			annotation:       kubeletAPILimitedReaderAnnotationKey,
			autopilotEnabled: true,
			expectResult: kubeletReadonlyCSRResponse{
				message: "bad request: length of pod name and UID are not equal",
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(fmt.Sprint(testCase.name), func(t *testing.T) {
			request := generateTestRequestByPodName(t, testCase.podNames, testCase.podUIDs, testCase.username)
			request.autopilotEnabled = testCase.autopilotEnabled
			createPodWithAnnotation(testCase.podNames, testCase.podUIDs, testCase.namespace, testCase.annotation, testCase.wantError, *request.controllerContext)
			response := validatePodAnnotation(request)
			compareKubeletReadonlyCSRRequestResult(response, testCase.expectResult, t)
		})
	}
}

func TestValidateRbac(t *testing.T) {
	testCases := []struct {
		name         string
		allowed      bool
		wantError    bool
		expectResult kubeletReadonlyCSRResponse
	}{
		{
			name: "not allow to create csr",
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
		{
			name:      "error to create csr",
			allowed:   true,
			wantError: true,
			expectResult: kubeletReadonlyCSRResponse{
				result:  false,
				err:     fmt.Errorf("failed to get subject access review"),
				message: "subject access review request failed: failed to get subject access review",
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(fmt.Sprint(testCase.name), func(t *testing.T) {
			request := generateTestRequestBySubjectAccessReview(t, testCase.allowed, testCase.wantError)
			request.csr.Spec.Username = "test"
			response := validateRbac(request)
			compareKubeletReadonlyCSRRequestResult(response, testCase.expectResult, t)
		})
	}
}

func TestValidateCommonName(t *testing.T) {
	testCases := []struct {
		name         string
		x509cr       *x509.CertificateRequest
		expectResult kubeletReadonlyCSRResponse
	}{
		{
			name: fmt.Sprintf("x509cr has prefix %s", kubeletReadonlyCRCommonName),
			x509cr: &x509.CertificateRequest{
				Subject: pkix.Name{
					CommonName:   kubeletReadonlyCRCommonName,
					Organization: []string{"testOrg"},
				},
			},
			expectResult: kubeletReadonlyCSRResponse{
				result:  true,
				err:     nil,
				message: fmt.Sprintf("x509 common name starts with %s", kubeletReadonlyCRCommonName),
			},
		},
		{
			name: fmt.Sprintf("x509cr does not has prefix %s", kubeletReadonlyCRCommonName),
			x509cr: &x509.CertificateRequest{
				Subject: pkix.Name{
					CommonName:   "test",
					Organization: []string{"testOrg"},
				},
			},
			expectResult: kubeletReadonlyCSRResponse{
				result:  false,
				err:     nil,
				message: fmt.Sprintf("x509 common name should start with %s", kubeletReadonlyCRCommonName),
			},
		},
		{
			name: "x509cr  has empty common name",
			x509cr: &x509.CertificateRequest{
				Subject: pkix.Name{
					CommonName:   "",
					Organization: []string{"testOrg"},
				},
			},
			expectResult: kubeletReadonlyCSRResponse{
				result:  false,
				err:     nil,
				message: "x509cr or CommonName is empty",
			},
		},
		{
			name: "x509 is empty",
			expectResult: kubeletReadonlyCSRResponse{
				result:  false,
				err:     nil,
				message: fmt.Sprintf("x509cr or CommonName is empty"),
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(fmt.Sprint(testCase.name), func(t *testing.T) {
			request := generateTestKubeletReadonlyCSRRequest(t)
			request.x509cr = testCase.x509cr
			response := validateCommonName(request)
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

func createPodWithAnnotation(podNames, podUIDs []string, namespace, annotation string, wantError bool, controllerContext controllerContext) {
	if len(podNames) != len(podUIDs) {
		return
	}
	podsMap := map[string]*v1.Pod{}
	for index, podName := range podNames {
		podsMap[podName] = &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      podName,
				UID:       types.UID(podUIDs[index]),
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
	}
	client := controllerContext.client.(*fake.Clientset)
	if !wantError {
		client.PrependReactor("get", "pods", func(action testclient.Action) (handled bool, ret runtime.Object, err error) {

			return true, podsMap[action.(testclient.GetAction).GetName()], nil
		})
	} else {
		client.PrependReactor("get", "pods", func(action testclient.Action) (handled bool, ret runtime.Object, err error) {
			return true, nil, fmt.Errorf("failed to get pod")
		})
	}
}

type kubeletReadonlyTestRequestBuilder struct {
	sarAllowed             bool
	podAnnotationAllowed   bool
	usageAllowed           bool
	autopilotEnabled       bool
	sarWantError           bool
	podAnnotationWantError bool
	commonName             string
}

func generateTestRequest(t *testing.T, builder *kubeletReadonlyTestRequestBuilder) kubeletReadonlyCSRRequest {

	request := generateTestRequestBySubjectAccessReview(t, builder.sarAllowed, builder.sarWantError)
	request.csr.Spec.Username = "system:serviceaccount:default:test"
	request.csr.Spec.Extra = map[string]capi.ExtraValue{}
	request.autopilotEnabled = builder.autopilotEnabled
	if builder.podAnnotationAllowed {
		request.csr.Spec.Extra = map[string]capi.ExtraValue{
			podNameKey: []string{"test"},
			podUIDKey:  []string{"test-UID"},
		}
		createPodWithAnnotation([]string{"test"}, []string{"test-UID"}, "default", kubeletAPILimitedReaderAnnotationKey, builder.podAnnotationWantError, *request.controllerContext)
	}
	if builder.usageAllowed {
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
	pk, err := ecdsa.GenerateKey(elliptic.P224(), insecureRand)
	if err != nil {
		t.Fatalf("failed to generate public key, error: %v", err)
	}
	csrb, err := x509.CreateCertificateRequest(rand.New(rand.NewSource(0)), &x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName:   builder.commonName,
			Organization: []string{"testOrg"},
		},
	}, pk)
	if err != nil {
		t.Fatalf("failed to generate CreateCertificateRequest, error: %v", err)
	}
	blocks := [][]byte{pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrb})}

	request.csr.Spec.Request = bytes.TrimSpace(bytes.Join(blocks, nil))

	return request
}

func generateTestRequestByPodName(t *testing.T, podNames, podUIDs []string, username string) kubeletReadonlyCSRRequest {
	request := generateTestKubeletReadonlyCSRRequest(t)
	request.csr.Spec.Extra = map[string]capi.ExtraValue{
		"annotations": []string{"test"},
	}
	if len(podNames) != 0 {
		request.csr.Spec.Extra[podNameKey] = podNames
	}

	if len(podUIDs) != 0 {
		request.csr.Spec.Extra[podUIDKey] = podUIDs
	}

	if len(username) != 0 {
		request.csr.Spec.Username = username
	}
	return request
}

func generateTestRequestBySubjectAccessReview(t *testing.T, allowed, wantError bool) kubeletReadonlyCSRRequest {
	request := generateTestKubeletReadonlyCSRRequest(t)
	client := request.controllerContext.client.(*fake.Clientset)
	if !wantError {
		client.PrependReactor("create", "subjectaccessreviews", func(action testclient.Action) (handled bool, ret runtime.Object, err error) {
			return true, &authorization.SubjectAccessReview{
				Status: authorization.SubjectAccessReviewStatus{
					Allowed: allowed,
				},
			}, nil
		})
	} else {
		client.PrependReactor("create", "subjectaccessreviews", func(action testclient.Action) (handled bool, ret runtime.Object, err error) {
			return true, nil, fmt.Errorf("failed to get subject access review")
		})
	}
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
