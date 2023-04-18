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

package hms

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

type testHMS struct {
	server     *httptest.Server
	saMappings map[serviceAccountMapping]bool
	req        []byte
	isSyncCall bool
	wantErr    bool
}

func newTestHMS(saMappings map[serviceAccountMapping]bool, wantErr, isSyncCall bool) *testHMS {
	hms := &testHMS{saMappings: saMappings, wantErr: wantErr, isSyncCall: isSyncCall}
	hms.server = httptest.NewServer(hms)
	return hms
}

// ServeHTTP returns an error when wantErr is true.
// For Sync(), it simply returns.
// For Authorize(), it uses saMappings to populate the permitted/denied mappings.
func (hms *testHMS) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if hms.wantErr {
		http.Error(w, "random error message", http.StatusBadRequest)
		return
	}
	var err error
	hms.req, err = io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("error reading request: %v", err), http.StatusInternalServerError)
		return
	}
	if hms.isSyncCall {
		return
	}
	var req authorizeSAMappingRequest
	if err := json.Unmarshal(hms.req, &req); err != nil {
		http.Error(w, fmt.Sprintf("error unmarshalling request: %v", err), http.StatusInternalServerError)
		return
	}
	var resp authorizeSAMappingResponse
	for _, m := range req.RequestedMappings {
		if hms.saMappings[m] {
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
		{KSAName: deniedKSA, KNSName: testKNS, GSAEmail: testGSA}:    false,
		{KSAName: permittedKSA, KNSName: testKNS, GSAEmail: testGSA}: true,
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
			hmsServer := newTestHMS(saMappings, tc.wantErr, false)
			hmsClient, err := NewClient(hmsServer.server.URL, nil)
			if err != nil {
				t.Fatalf("Error creating test HMS client: %v", err)
			}

			permitted, err := hmsClient.Authorize(context.Background(), testKNS, tc.ksa, testGSA)
			if got, want := err != nil, tc.wantErr; got != want {
				t.Errorf("hmsClient.Authorize()=%v, want err: %v", err, want)
			}
			if got, want := permitted, tc.wantPermitted; got != want {
				t.Errorf("hmsClient.Authorize()=%v: want permitted: %v", err, want)
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
				NodeName:  node,
				NodeZone:  zone,
				GSAEmails: gsaList,
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			hmsServer := newTestHMS(nil, tc.wantErr, true)
			hmsClient, err := NewClient(hmsServer.server.URL, nil)
			if err != nil {
				t.Fatalf("Error creating test HMS client: %v", err)
			}
			err = hmsClient.Sync(context.Background(), tc.wantReq.NodeName, tc.wantReq.NodeZone, tc.wantReq.GSAEmails)
			if got, want := err != nil, tc.wantErr; got != want {
				t.Errorf("hmsClient.Sync()=%v, want err: %v", err, want)
			}
			if err != nil {
				return
			}
			var gotReq syncNodeRequest
			if err := json.Unmarshal(hmsServer.req, &gotReq); err != nil {
				t.Errorf("Got invalid HMS request %v: %v", hmsServer.req, err)
			}
			if got, want := gotReq, tc.wantReq; !reflect.DeepEqual(got, want) {
				t.Errorf("hmsClient.Sync()=%v: want: %v", got, want)
			}
		})
	}
}
