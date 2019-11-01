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
	"reflect"
	"testing"

	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	lister "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"net/http"
	"net/http/httptest"
)

type fakeSALister struct {
	lister.ServiceAccountLister
	ksa *core.ServiceAccount
	err error
}

func (f fakeSALister) ServiceAccounts(namespace string) lister.ServiceAccountNamespaceLister {
	return &f
}

func (f fakeSALister) Get(name string) (*core.ServiceAccount, error) {
	return f.ksa, f.err
}

func newKSA(sa serviceAccount, gsa gsaEmail) *core.ServiceAccount {
	ksa := &core.ServiceAccount{
		TypeMeta: meta.TypeMeta{
			Kind:       "ServiceAccount",
			APIVersion: "v1",
		},
		ObjectMeta: meta.ObjectMeta{
			Name:        sa.name,
			Namespace:   sa.namespace,
			Annotations: make(map[string]string),
		},
	}
	if gsa != "" {
		ksa.ObjectMeta.Annotations["iam.gke.io/gcp-service-account"] = string(gsa)
	}
	return ksa
}

type testHMS struct {
	server *httptest.Server
	resp   interface{}
	ok     bool
}

func newTestHMS(resp interface{}, ok bool) *testHMS {
	hms := &testHMS{resp: resp, ok: ok}
	hms.server = httptest.NewServer(hms)
	return hms
}

// ServeHTTP implements the http.Handler interface.
func (hms *testHMS) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !hms.ok {
		http.Error(w, "random error message", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(hms.resp)
}

func TestServiceAccountVerify(t *testing.T) {
	testSA := serviceAccount{"foo", "bar"}
	testGSA := gsaEmail("bar@testproject.iam.gserviceaccount.com")
	testKSA := newKSA(testSA, testGSA)
	testHMSSAMap := serviceAccountMapping{
		KNSName:  testSA.namespace,
		KSAName:  testSA.name,
		GSAEmail: string(testGSA),
	}
	testHMSRespPermitted := &authorizeSAMappingResponse{
		PermittedMappings: []serviceAccountMapping{testHMSSAMap},
	}
	testHMSRespDenied := &authorizeSAMappingResponse{
		DeniedMappings: []serviceAccountMapping{testHMSSAMap},
	}

	otherSA := serviceAccount{"otherNamespace", "otherName"}
	otherGSA := gsaEmail("othergsa@testproject.iam.gserviceaccount.com")
	mapWithBothSA := map[serviceAccount]gsaEmail{
		testSA:  testGSA,
		otherSA: otherGSA,
	}
	mapHasOnlyTestSA := map[serviceAccount]gsaEmail{
		testSA: testGSA,
	}
	mapHasOnlyOtherSA := map[serviceAccount]gsaEmail{
		otherSA: otherGSA,
	}

	tests := []struct {
		desc           string
		ksa            *core.ServiceAccount
		ksaErr         error
		ksaKeyOverride string
		initMap        map[serviceAccount]gsaEmail
		hmsResp        interface{}
		hmsOK          bool
		wantSync       bool
		wantErr        bool
		wantMap        map[serviceAccount]gsaEmail
	}{
		{
			desc:     "newly permitted",
			ksa:      testKSA,
			hmsResp:  testHMSRespPermitted,
			hmsOK:    true,
			initMap:  mapHasOnlyOtherSA,
			wantMap:  mapWithBothSA,
			wantSync: true,
		},
		{
			desc:    "already permitted",
			ksa:     testKSA,
			hmsResp: testHMSRespPermitted,
			hmsOK:   true,
			initMap: mapWithBothSA,
			wantMap: mapWithBothSA,
		},
		{
			desc:    "denied",
			ksa:     testKSA,
			hmsResp: testHMSRespDenied,
			hmsOK:   true,
			initMap: mapHasOnlyOtherSA,
			wantMap: mapHasOnlyOtherSA,
		},
		{
			desc:     "previous permission invalidated",
			ksa:      testKSA,
			hmsResp:  testHMSRespDenied,
			hmsOK:    true,
			initMap:  mapWithBothSA,
			wantMap:  mapHasOnlyOtherSA,
			wantSync: true,
		},
		{
			desc:    "ksa not found",
			ksa:     testKSA,
			ksaErr:  fmt.Errorf("not found"),
			initMap: mapHasOnlyOtherSA,
			wantMap: mapHasOnlyOtherSA,
			wantErr: true,
		},
		{
			desc:           "bad key",
			ksa:            testKSA,
			ksaKeyOverride: "invalid key",
			initMap:        mapHasOnlyOtherSA,
			wantMap:        mapHasOnlyOtherSA,
			wantErr:        true,
		},
		{
			desc:    "hms returned http error",
			ksa:     testKSA,
			initMap: mapHasOnlyOtherSA,
			wantMap: mapHasOnlyOtherSA,
			wantErr: true,
		},
		{
			desc:    "hms returned corrupted response",
			ksa:     testKSA,
			hmsResp: "a very bad response",
			hmsOK:   true,
			initMap: mapHasOnlyOtherSA,
			wantMap: mapHasOnlyOtherSA,
			wantErr: true,
		},
		{
			desc:    "missing gsa annotation",
			ksa:     newKSA(testSA, ""),
			initMap: mapHasOnlyOtherSA,
			wantMap: mapHasOnlyOtherSA,
		},
		{
			desc:     "previous permission invalided by annotation removal",
			ksa:      newKSA(testSA, ""),
			initMap:  mapWithBothSA,
			wantMap:  mapHasOnlyOtherSA,
			wantSync: true,
		},
		{
			desc: "previous permission invalided by new annotation",
			ksa:  newKSA(testSA, "denied_gsa"),
			hmsResp: &authorizeSAMappingResponse{
				DeniedMappings: []serviceAccountMapping{
					{
						KNSName:  testSA.namespace,
						KSAName:  testSA.name,
						GSAEmail: "denied_gsa",
					},
				},
			},
			hmsOK:    true,
			initMap:  mapWithBothSA,
			wantMap:  mapHasOnlyOtherSA,
			wantSync: true,
		},
		{
			desc: "previous permission replaced by new annotation",
			ksa:  newKSA(testSA, "new_permitted_gsa"),
			hmsResp: &authorizeSAMappingResponse{
				PermittedMappings: []serviceAccountMapping{
					{
						KNSName:  testSA.namespace,
						KSAName:  testSA.name,
						GSAEmail: "new_permitted_gsa",
					},
				},
			},
			hmsOK:   true,
			initMap: mapHasOnlyTestSA,
			wantMap: map[serviceAccount]gsaEmail{
				testSA: gsaEmail("new_permitted_gsa"),
			},
			wantSync: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			c := fake.NewSimpleClientset(tc.ksa)
			hmsServer := newTestHMS(tc.hmsResp, tc.hmsOK)
			hmsClient, err := newHMSClient(hmsServer.server.URL, nil)
			if err != nil {
				t.Fatalf("error creating test HMS client: %v", err)
			}
			m := newSAMap()
			for sa, gsa := range tc.initMap {
				m.add(sa, gsa)
			}
			sav := &serviceAccountVerifier{
				c:           c,
				sals:        fakeSALister{ksa: tc.ksa, err: tc.ksaErr},
				hms:         hmsClient,
				verifiedSAs: m,
			}
			var key string
			if tc.ksaKeyOverride != "" {
				key = tc.ksaKeyOverride
			} else {
				key, err = cache.MetaNamespaceKeyFunc(tc.ksa)
				if err != nil {
					t.Fatalf("error getting key for KSA %+v: %v", tc.ksa, err)
				}
			}

			sync, err := sav.verify(key)

			if tc.wantErr && err == nil {
				t.Error("expecting but did not get an error")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("not expecting error but got %v", err)
			}
			if !reflect.DeepEqual(sav.verifiedSAs.ma, tc.wantMap) {
				t.Errorf("got map: %v\nwant map: %v", sav.verifiedSAs.ma, tc.wantMap)
			}
			if sync != tc.wantSync {
				t.Errorf("got sync: %v\nwant sync: %v", sync, tc.wantSync)
			}
		})
	}
}
