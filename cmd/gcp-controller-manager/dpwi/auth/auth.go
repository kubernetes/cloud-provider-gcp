/*
Copyright 2023 The Kubernetes Authors.

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

// Package auth provides clients to call Auth service to sync node and authorize SA.
package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	apiserveroptions "k8s.io/apiserver/pkg/server/options"
	"k8s.io/apiserver/pkg/util/webhook"
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const (
	authRequestTimeout = 30 * time.Second
)

// Client is an Auth service client.
type Client struct {
	webhook *webhook.GenericWebhook
	useAuth bool
}

// NewClient creates a new client with the server url and the AuthProviderConfig.
func NewClient(url, hmsURL string, authProvider *clientcmdapi.AuthProviderConfig) (*Client, error) {
	useAuth := url != ""
	if !useAuth {
		url = hmsURL
	}
	config := &rest.Config{
		Host:         url,
		AuthProvider: authProvider,
		Timeout:      authRequestTimeout,
		QPS:          50,
		Burst:        100,
		ContentConfig: rest.ContentConfig{
			NegotiatedSerializer: serializer.NegotiatedSerializerWrapper(runtime.SerializerInfo{}),
		},
	}
	rc, err := rest.UnversionedRESTClientFor(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create REST client for HMS from config %v: %w", config, err)
	}
	return &Client{
		webhook: &webhook.GenericWebhook{RestClient: rc, RetryBackoff: *apiserveroptions.DefaultAuthWebhookRetryBackoff(), ShouldRetry: webhook.DefaultShouldRetry},
		useAuth: useAuth,
	}, nil
}

// isErrorHTTPStatus returns true if statusCode is not in the range of 200 to 299 inclusive.
func isErrorHTTPStatus(statusCode int) bool {
	return statusCode < 200 || statusCode >= 300
}

// Sync syncs gsaList on a node in a specific zone.
func (h *Client) Sync(ctx context.Context, node, zone string, gsaList []string) error {
	sort.Strings(gsaList)
	if !h.useAuth {
		return h.call(ctx, hmsSyncNodeRequest{
			NodeName:  node,
			NodeZone:  zone,
			GSAEmails: gsaList,
		}, nil)
	}
	return h.call(ctx, syncNodeRequest{
		Node:                  node,
		NodeZone:              zone,
		GoogleServiceAccounts: gsaList,
	}, nil)
}

// Authorize implements the saMappingAuthorizer interface.  It calls HMS to verify if ksa has
// permission to get certificates as gsa.
func (h *Client) Authorize(ctx context.Context, kns, ksa, gsa string) (bool, error) {
	if !h.useAuth {
		reqMapping := hmsServiceAccountMapping{
			KNSName:  kns,
			KSAName:  ksa,
			GSAEmail: gsa,
		}
		var rsp hmsAuthorizeSAMappingResponse
		if err := h.call(ctx, hmsAuthorizeSAMappingRequest{
			RequestedMappings: []hmsServiceAccountMapping{reqMapping},
		}, &rsp); err != nil {
			return false, err
		}
		if permitted := rsp.PermittedMappings; len(permitted) > 0 && permitted[0] == reqMapping {
			return true, nil
		}
		if denied := rsp.DeniedMappings; len(denied) > 0 && denied[0] == reqMapping {
			return false, nil
		}
		return false, fmt.Errorf("internal error: requested mapping %v not found in response %+v", reqMapping, rsp)
	}

	reqMapping := serviceAccountMapping{
		KubernetesNamespace:      kns,
		KubernetesServiceAccount: ksa,
		GoogleServiceAccount:     gsa,
	}
	var rsp authorizeSAMappingResponse
	if err := h.call(ctx, authorizeSAMappingRequest{
		RequestedMappings: []serviceAccountMapping{reqMapping},
	}, &rsp); err != nil {
		return false, err
	}
	if permitted := rsp.PermittedMappings; len(permitted) > 0 && permitted[0] == reqMapping {
		return true, nil
	}
	if denied := rsp.DeniedMappings; len(denied) > 0 && denied[0] == reqMapping {
		return false, nil
	}
	return false, fmt.Errorf("internal error: requested mapping %v not found in response %+v", reqMapping, rsp)
}

func (h *Client) call(ctx context.Context, req, rsp interface{}) error {
	enc, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to encode %v: %w", req, err)
	}

	result := h.webhook.WithExponentialBackoff(ctx, func() rest.Result {
		return h.webhook.RestClient.Post().Body(enc).Do(ctx)
	})

	if err = result.Error(); err != nil {
		return fmt.Errorf("error resulted from request %v: %w", req, err)
	}

	// result.StatusCode is set only if result.Error is nil.
	var status int
	result.StatusCode(&status)
	if isErrorHTTPStatus(status) {
		return fmt.Errorf("unsuccessful status code resulted from request %v: %d", req, status)
	}

	if rsp == nil {
		return nil
	}
	raw, err := result.Raw()
	if err != nil {
		return fmt.Errorf("request succeeed but failed to read response: %w", err)
	}
	if err = json.Unmarshal(raw, rsp); err != nil {
		return fmt.Errorf("request succeeed but got error %w parsing response: %q", err, raw)
	}
	return nil
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
	KubernetesNamespace string `json:"kubernetesNamespace"`

	// Name of a Kubernetes Service Account namespaced under kns_name.
	KubernetesServiceAccount string `json:"kubernetesServiceAccount"`

	// Email address of a GCP Service Account; that is,
	// <gsa_name>@<project_name>.iam.gserviceaccount.com.
	GoogleServiceAccount string `json:"googleServiceAccount"`
}

// hmsAuthorizeSAMappingRequest is the request message for the authorizeSAMapping RPC.
type hmsAuthorizeSAMappingRequest struct {
	// List of KSA to GSA mappings to be authorized.
	RequestedMappings []hmsServiceAccountMapping `json:"requestedMappings"`
}

// hmsAuthorizeSAMappingResponse is the response message for the authorizeSAMapping RPC.
type hmsAuthorizeSAMappingResponse struct {
	// List of KSA to GSA mappings from authorizeSAMappingRequest.requested_mappings that are
	// denied.
	DeniedMappings []hmsServiceAccountMapping `json:"deniedMappings"`

	// List of KSA to GSA mappings from authorizeSAMappingRequest.requested_mappings that are
	// permitted.
	PermittedMappings []hmsServiceAccountMapping `json:"permittedMappings"`
}

// serviceAccountMapping specifies mapping of a Kubernetes Service Account to a GCP Service Account.
type hmsServiceAccountMapping struct {
	// Name of a Kubernetes Namespace for ksa_name.
	KNSName string `json:"knsName"`

	// Name of a Kubernetes Service Account namespaced under kns_name.
	KSAName string `json:"ksaName"`

	// Email address of a GCP Service Account; that is,
	// <gsa_name>@<project_name>.iam.gserviceaccount.com.
	GSAEmail string `json:"gsaEmail"`
}

// Request for SyncNode RPC of Auth service.
type syncNodeRequest struct {
	// Name of the zone for the node being synchronized.
	NodeZone string `json:"nodeZone"`
	// Name of the Kubernetes Node to be synchronized.
	Node string `json:"node"`
	// List of GCP Service Accounts for the Node in Email address format; that is,
	// <gsa_name>@<project_name>.iam.gserviceaccount.com.
	GoogleServiceAccounts []string `json:"googleServiceAccounts"`
}

// Request for SyncNode RPC of HMS.
type hmsSyncNodeRequest struct {
	// Name of the Kubernetes Node to be synchronized.
	NodeName string `json:"nodeName"`
	// List of GCP Service Accounts for the Node in Email address format; that is,
	// <gsa_name>@<project_name>.iam.gserviceaccount.com.
	GSAEmails []string `json:"gsaEmails"`
	// Name of the zone for the node being synchronized.
	NodeZone string `json:"nodeZone"`
}
