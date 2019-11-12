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

package main

import (
	"encoding/json"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apiserver/pkg/util/webhook"
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const (
	hmsRetryBackoff   = 500 * time.Millisecond
	hmsRequestTimeout = 30 * time.Second
)

type hmsClient struct {
	webhook *webhook.GenericWebhook
}

func newHMSClient(url string, authProvider *clientcmdapi.AuthProviderConfig) (*hmsClient, error) {
	config := &rest.Config{
		Host:         url,
		AuthProvider: authProvider,
		Timeout:      hmsRequestTimeout,
		ContentConfig: rest.ContentConfig{
			NegotiatedSerializer: serializer.NegotiatedSerializerWrapper(runtime.SerializerInfo{}),
		},
	}
	client, err := rest.UnversionedRESTClientFor(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create REST client for HMS from config %v: %v", config, err)
	}
	return &hmsClient{
		webhook: &webhook.GenericWebhook{client, hmsRetryBackoff},
	}, nil
}

// isErrorHTTPStatus returns true if statusCode is not in the range of 200 to 299 inclusive.
func isErrorHTTPStatus(statusCode int) bool {
	return statusCode < 200 || statusCode >= 300
}

// Authorize implements the saMappingAuthorizer interface.  It calls HMS to verify if ksa has
// permission to get certificates as gsa.
func (h *hmsClient) authorize(ksa serviceAccount, gsa gsaEmail) (bool, error) {
	reqMapping := serviceAccountMapping{
		KNSName:  ksa.Namespace,
		KSAName:  ksa.Name,
		GSAEmail: string(gsa),
	}
	req := authorizeSAMappingRequest{
		RequestedMappings: []serviceAccountMapping{reqMapping},
	}
	enc, err := json.Marshal(req)
	if err != nil {
		return false, fmt.Errorf("failed to encode %v: %v", req, err)
	}

	result := h.webhook.WithExponentialBackoff(func() rest.Result {
		return h.webhook.RestClient.Post().Body(enc).Do()
	})

	if err = result.Error(); err != nil {
		return false, fmt.Errorf("error resulted from request %v: %v", req, err)
	}

	// result.StatusCode is set only if result.Error is nil.
	var status int
	result.StatusCode(&status)
	if isErrorHTTPStatus(status) {
		return false, fmt.Errorf("unsuccessful status code resulted from request %v: %d", req, status)
	}

	raw, err := result.Raw()
	if err != nil {
		return false, fmt.Errorf("request succeeed but failed to read response: %v", err)
	}
	var rsp authorizeSAMappingResponse
	if err = json.Unmarshal(raw, &rsp); err != nil {
		return false, fmt.Errorf("request succeeed but got error %v parsing response: %q", err, raw)
	}
	if permitted := rsp.PermittedMappings; len(permitted) > 0 && permitted[0] == reqMapping {
		return true, nil
	}
	if denied := rsp.DeniedMappings; len(denied) > 0 && denied[0] == reqMapping {
		return false, nil
	}
	return false, fmt.Errorf("internal error: requested mapping %v not found in response %+v", reqMapping, rsp)
}

// authorizeSAMappingRequest is the request message for the authorizeSAMapping RPC.
type authorizeSAMappingRequest struct {
	// List of KSA to GSA mappings to be authorized.
	RequestedMappings []serviceAccountMapping `json:"requestedMappings"`
}

// authorizeSAMappingResponse is the response message for the authorizeSAMapping RPC.
type authorizeSAMappingResponse struct {
	// List of KSA to GSA mappings from authorizeSAMappingRequest.requested_mappings that are
	// denied.
	DeniedMappings []serviceAccountMapping `json:"deniedMappings"`

	// List of KSA to GSA mappings from authorizeSAMappingRequest.requested_mappings that are
	// permitted.
	PermittedMappings []serviceAccountMapping `json:"permittedMappings"`
}

// serviceAccountMapping specifies mapping of a Kubernetes Service Account to a GCP Service Account.
type serviceAccountMapping struct {
	// Name of a Kubernetes Namespace for ksa_name.
	KNSName string `json:"knsName"`

	// Name of a Kubernetes Service Account namespaced under kns_name.
	KSAName string `json:"ksaName"`

	// Email address of a GCP Service Account; that is,
	// <gsa_name>@<project_name>.iam.gserviceaccount.com.
	GSAEmail string `json:"gsaEmail"`
}
