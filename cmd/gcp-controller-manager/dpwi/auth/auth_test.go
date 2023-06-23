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

package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

const (
	testGSA = "gsa@testproject.iam.gserviceaccount.com"
)

type testAuth struct {
	server     *httptest.Server
	saMappings map[serviceAccountMapping]bool
	req        []byte
	isSyncCall bool
	wantErr    bool
}

func newTestAuth(saMappings map[serviceAccountMapping]bool, wantErr, isSyncCall bool) *testAuth {
	auth := &testAuth{saMappings: saMappings, wantErr: wantErr, isSyncCall: isSyncCall}
	auth.server = httptest.NewServer(auth)
	return auth
}

// ServeHTTP returns an error when wantErr is true.
// For Sync(), it simply returns.
// For Authorize(), it uses saMappings to populate the permitted/denied mappings.
func (auth *testAuth) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if auth.wantErr {
		http.Error(w, "random error message", http.StatusBadRequest)
		return
	}
	var err error
	auth.req, err = io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("error reading request: %v", err), http.StatusInternalServerError)
		return
	}
	if auth.isSyncCall {
		return
	}
	var req authorizeSAMappingRequest
	if err := json.Unmarshal(auth.req, &req); err != nil {
		http.Error(w, fmt.Sprintf("error unmarshalling request: %v", err), http.StatusInternalServerError)
		return
	}
	var resp authorizeSAMappingResponse
	for _, m := range req.RequestedMappings {
		if auth.saMappings[m] {
			resp.PermittedMappings = append(resp.PermittedMappings, m)
		} else {
			resp.DeniedMappings = append(resp.DeniedMappings, m)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func TestAuthorize(t *testing.T) {
	testKNS := "testKNS"
	permittedKSA := "permittedKSA"
	deniedKSA := "deniedKSA"
	saMappings := map[serviceAccountMapping]bool{
		{KubernetesServiceAccount: deniedKSA, KubernetesNamespace: testKNS, GoogleServiceAccount: testGSA}:    false,
		{KubernetesServiceAccount: permittedKSA, KubernetesNamespace: testKNS, GoogleServiceAccount: testGSA}: true,
	}

	tests := []struct {
		desc          string
		ksa           string
		wantPermitted bool
		wantErr       bool
	}{
		{
			desc:    "expect errors",
			wantErr: true,
		},
		{
			desc:          "permitted ksa",
			ksa:           permittedKSA,
			wantPermitted: true,
		},
		{
			desc: "denied ksa",
			ksa:  deniedKSA,
		},
	}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			authServer := newTestAuth(saMappings, tc.wantErr, false)
			authClient, err := NewClient(authServer.server.URL, "", nil)
			if err != nil {
				t.Fatalf("Error creating test Auth client: %v", err)
			}

			permitted, err := authClient.Authorize(context.Background(), testKNS, tc.ksa, testGSA)
			if got, want := err != nil, tc.wantErr; got != want {
				t.Errorf("authClient.Authorize()=%v, want err: %v", err, want)
			}
			if got, want := permitted, tc.wantPermitted; got != want {
				t.Errorf("authClient.Authorize()=%v: want permitted: %v", err, want)
			}
		})
	}
}

func TestSync(t *testing.T) {
	node := "testNode"
	zone := "testZone"
	gsaList := []string{testGSA}
	tests := []struct {
		desc    string
		wantErr bool
		wantReq syncNodeRequest
	}{
		{
			desc:    "expect errors",
			wantErr: true,
		},
		{
			desc: "permitted ksa",
			wantReq: syncNodeRequest{
				Node:                  node,
				NodeZone:              zone,
				GoogleServiceAccounts: gsaList,
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			authServer := newTestAuth(nil, tc.wantErr, true)
			authClient, err := NewClient(authServer.server.URL, "", nil)
			if err != nil {
				t.Fatalf("Error creating test Auth client: %v", err)
			}
			err = authClient.Sync(context.Background(), tc.wantReq.Node, tc.wantReq.NodeZone, tc.wantReq.GoogleServiceAccounts)
			if got, want := err != nil, tc.wantErr; got != want {
				t.Errorf("authClient.Sync()=%v, want err: %v", err, want)
			}
			if err != nil {
				return
			}
			var gotReq syncNodeRequest
			if err := json.Unmarshal(authServer.req, &gotReq); err != nil {
				t.Errorf("Got invalid Auth request %v: %v", authServer.req, err)
			}
			if got, want := gotReq, tc.wantReq; !reflect.DeepEqual(got, want) {
				t.Errorf("authClient.Sync()=%v: want: %v", got, want)
			}
		})
	}
}
