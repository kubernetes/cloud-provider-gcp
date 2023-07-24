/* Copyright 2023 The Kubernetes Authors.

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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
)

// FakeServer is used for testing.
type FakeServer struct {
	// Server is a httptest server.
	Server     *httptest.Server
	saMappings map[string]string
	// SyncGSAs records the GSAs per node name for Sync call.
	SyncGSAs map[string][]string
	// SyncCount records the sync call count per node name.
	SyncCount map[string]int
	// AuthorizeCount records the authorize call count per KSA.
	AuthorizeCount map[string]int
}

// NewFakeServer creates a new FakeServer for testing.
func NewFakeServer(saMappings map[string]string) *FakeServer {
	auth := &FakeServer{
		saMappings:     saMappings,
		SyncGSAs:       make(map[string][]string),
		SyncCount:      make(map[string]int),
		AuthorizeCount: make(map[string]int),
	}
	auth.Server = httptest.NewServer(auth)
	return auth
}

// ServeHTTP implements the http.Handler interface.
func (f *FakeServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("error reading request: %v", err), http.StatusInternalServerError)
		return
	}
	var syncReq syncNodeRequest
	if err := json.Unmarshal(raw, &syncReq); err == nil && syncReq.Node != "" {
		n := syncReq.Node
		f.SyncGSAs[n] = syncReq.GoogleServiceAccounts
		f.SyncCount[n]++
		return
	}
	var req authorizeSAMappingRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		http.Error(w, fmt.Sprintf("error unmarshalling request: %v", err), http.StatusInternalServerError)
		return
	}
	var resp authorizeSAMappingResponse
	for _, m := range req.RequestedMappings {
		key := fmt.Sprintf("%s/%s", m.KubernetesNamespace, m.KubernetesServiceAccount)
		f.AuthorizeCount[key]++
		if f.saMappings[key] == m.GoogleServiceAccount {
			resp.PermittedMappings = append(resp.PermittedMappings, m)
		} else {
			resp.DeniedMappings = append(resp.DeniedMappings, m)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
