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
	"bytes"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"testing"
	"text/template"

	capi "k8s.io/api/certificates/v1"
	certsv1 "k8s.io/api/certificates/v1"
	certsv1b1 "k8s.io/api/certificates/v1beta1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"
)

const kubeConfigTmpl = `
clusters:
- cluster:
    server: {{ .Server }}
    name: testcluster
users:
- user:
    username: admin
    password: mypass
`

var (
	statusApproved = capi.CertificateSigningRequestStatus{
		Conditions: []capi.CertificateSigningRequestCondition{
			capi.CertificateSigningRequestCondition{Type: capi.CertificateApproved},
		},
	}
)

func generateCSR() []byte {
	// noncryptographic for faster testing
	// DO NOT COPY THIS CODE
	insecureRand := rand.New(rand.NewSource(0))

	keyBytes, err := rsa.GenerateKey(insecureRand, 1024)
	if err != nil {
		panic("error generating key")
	}

	csrTemplate := &x509.CertificateRequest{}

	csrBytes, err := x509.CreateCertificateRequest(insecureRand, csrTemplate, keyBytes)
	if err != nil {
		panic("error creating CSR")
	}

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrBytes})
}

func TestGKESigner(t *testing.T) {
	goodResponse := &certsv1.CertificateSigningRequest{
		Status: certsv1.CertificateSigningRequestStatus{
			Certificate: []byte("fake certificate"),
		},
	}

	invalidResponse := "{ \"status\": \"Not a properly formatted CSR response\" }"

	cases := []struct {
		name          string
		csr           *capi.CertificateSigningRequest
		mockResponse  interface{}
		expected      []byte
		failCalls     int
		wantProcessed bool
		wantErr       bool
	}{
		{
			name: "Signs approved certs with nil signer name",
			csr: &certsv1.CertificateSigningRequest{
				Spec: capi.CertificateSigningRequestSpec{
					SignerName: "",
					Request:    generateCSR(),
				},
				Status: statusApproved,
			},
			mockResponse:  goodResponse,
			expected:      goodResponse.Status.Certificate,
			wantProcessed: true,
			wantErr:       false,
		},
		{
			name: "Signs Approved API client certificates",
			csr: &certsv1.CertificateSigningRequest{
				Spec: capi.CertificateSigningRequestSpec{
					SignerName: certsv1.KubeAPIServerClientSignerName,
					Request:    generateCSR(),
				},
				Status: statusApproved,
			},
			mockResponse:  goodResponse,
			expected:      goodResponse.Status.Certificate,
			wantProcessed: true,
			wantErr:       false,
		},
		{
			name: "Signs kubelet client certificates",
			csr: &certsv1.CertificateSigningRequest{
				Spec: capi.CertificateSigningRequestSpec{
					SignerName: certsv1.KubeAPIServerClientKubeletSignerName,
					Request:    generateCSR(),
				},
				Status: statusApproved,
			},
			mockResponse:  goodResponse,
			expected:      goodResponse.Status.Certificate,
			wantProcessed: true,
			wantErr:       false,
		},
		{
			name: "Signs kubelet serving certificates",
			csr: &certsv1.CertificateSigningRequest{
				Spec: capi.CertificateSigningRequestSpec{
					SignerName: certsv1.KubeletServingSignerName,
					Request:    generateCSR(),
				},
				Status: statusApproved,
			},
			mockResponse:  goodResponse,
			expected:      goodResponse.Status.Certificate,
			wantProcessed: true,
			wantErr:       false,
		},
		{
			name: "Signs legacy-unknown certificates",
			csr: &certsv1.CertificateSigningRequest{
				Spec: capi.CertificateSigningRequestSpec{
					SignerName: certsv1b1.LegacyUnknownSignerName,
					Request:    generateCSR(),
				},
				Status: statusApproved,
			},
			mockResponse:  goodResponse,
			expected:      goodResponse.Status.Certificate,
			wantProcessed: true,
			wantErr:       false,
		},
		{
			name: "Signs API client certificates with a few failed calls",
			csr: &certsv1.CertificateSigningRequest{
				Spec: capi.CertificateSigningRequestSpec{
					SignerName: certsv1.KubeAPIServerClientSignerName,
					Request:    generateCSR(),
				},
				Status: statusApproved,
			},
			mockResponse:  goodResponse,
			expected:      goodResponse.Status.Certificate,
			failCalls:     3,
			wantProcessed: true,
			wantErr:       false,
		},
		{
			name:         "Returns error after many failed calls",
			mockResponse: goodResponse,
			csr: &certsv1.CertificateSigningRequest{
				Spec: capi.CertificateSigningRequestSpec{
					SignerName: certsv1.KubeAPIServerClientSignerName,
					Request:    generateCSR(),
				},
				Status: statusApproved,
			},
			failCalls:     20,
			wantProcessed: true,
			wantErr:       true,
		},
		{
			name: "Returns error after invalid response",
			csr: &certsv1.CertificateSigningRequest{
				Spec: capi.CertificateSigningRequestSpec{
					SignerName: certsv1.KubeAPIServerClientSignerName,
					Request:    generateCSR(),
				},
				Status: statusApproved,
			},
			mockResponse:  invalidResponse,
			wantProcessed: true,
			wantErr:       true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			server, err := newTestServer(c.mockResponse, c.failCalls)
			if err != nil {
				t.Fatalf("error creating test server")
			}

			kubeConfig, err := ioutil.TempFile("", "kubeconfig")
			if err != nil {
				t.Fatalf("error creating kubeconfig tempfile: %v", err)
			}

			tmpl, err := template.New("kubeconfig").Parse(kubeConfigTmpl)
			if err != nil {
				t.Fatalf("error creating kubeconfig template: %v", err)
			}

			data := struct{ Server string }{server.httpserver.URL}

			if err := tmpl.Execute(kubeConfig, data); err != nil {
				t.Fatalf("error executing kubeconfig template: %v", err)
			}

			if err := kubeConfig.Close(); err != nil {
				t.Fatalf("error closing kubeconfig template: %v", err)
			}

			ctlCtx := &controllerContext{
				client:                      fake.NewSimpleClientset(c.csr),
				clusterSigningGKEKubeconfig: kubeConfig.Name(),
				recorder:                    record.NewFakeRecorder(10),
			}
			signer, err := newGKESigner(ctlCtx)
			if err != nil {
				t.Fatalf("error creating GKESigner: %v", err)
			}

			processed, csrOut, err := signer.handleInternal(c.csr)

			if c.wantErr {
				if err == nil {
					t.Fatalf("wanted error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("wanted nil error, got %q", err)
			}

			if c.wantProcessed != processed {
				t.Fatalf("got processed=%v, want %v", processed, c.wantProcessed)
			}

			if !bytes.Equal(csrOut.Status.Certificate, c.expected) {
				t.Fatalf("response certificate didn't match expected %v: %v", c.expected, csrOut)
			}
		})
	}
}

type testServer struct {
	httpserver *httptest.Server
	failCalls  int
	response   interface{}
}

func newTestServer(response interface{}, failCalls int) (*testServer, error) {
	server := &testServer{
		response:  response,
		failCalls: failCalls,
	}

	server.httpserver = httptest.NewServer(server)
	return server, nil
}

func (s *testServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.failCalls > 0 {
		http.Error(w, "Service unavailable", 500)
		s.failCalls--
	} else {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(s.response)
	}
}
