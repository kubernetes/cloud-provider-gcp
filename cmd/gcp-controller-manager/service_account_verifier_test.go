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
	"io/ioutil"
	"reflect"
	"sync"
	"testing"

	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	"net/http"
	"net/http/httptest"
)

func newKSA(sa serviceAccount, gsa gsaEmail) *core.ServiceAccount {
	ksa := &core.ServiceAccount{
		TypeMeta: meta.TypeMeta{
			Kind:       "ServiceAccount",
			APIVersion: "v1",
		},
		ObjectMeta: meta.ObjectMeta{
			Name:        sa.Name,
			Namespace:   sa.Namespace,
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
	m      sync.Mutex
	req    []byte
	resp   interface{}
	ok     bool
}

func newTestHMS(resp interface{}, ok bool) *testHMS {
	hms := &testHMS{resp: resp, ok: ok}
	hms.server = httptest.NewServer(hms)
	return hms
}

func (hms *testHMS) getLastRequest() []byte {
	hms.m.Lock()
	defer hms.m.Unlock()
	return hms.req
}

// ServeHTTP implements the http.Handler interface.
func (hms *testHMS) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	hms.m.Lock()
	defer hms.m.Unlock()
	if !hms.ok {
		http.Error(w, "random error message", http.StatusInternalServerError)
		return
	}
	var err error
	hms.req, err = ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "error reading request", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(hms.resp)
}

func TestServiceAccountVerify(t *testing.T) {
	testSA := serviceAccount{"foo", "bar"}
	testSAKey := "foo/bar"
	testGSA := gsaEmail("bar@testproject.iam.gserviceaccount.com")
	testKSA := newKSA(testSA, testGSA)
	testHMSSAMap := serviceAccountMapping{
		KNSName:  testSA.Namespace,
		KSAName:  testSA.Name,
		GSAEmail: string(testGSA),
	}
	wantHMSReq := &authorizeSAMappingRequest{
		RequestedMappings: []serviceAccountMapping{testHMSSAMap},
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
		ksa            interface{}
		ksaErr         error
		ksaKeyOverride string
		initMap        map[serviceAccount]gsaEmail
		hmsResp        interface{}
		hmsOK          bool
		wantReq        *authorizeSAMappingRequest
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
			wantReq:  wantHMSReq,
			wantMap:  mapWithBothSA,
			wantSync: true,
		},
		{
			desc:    "already permitted",
			ksa:     testKSA,
			hmsResp: testHMSRespPermitted,
			hmsOK:   true,
			initMap: mapWithBothSA,
			wantReq: wantHMSReq,
			wantMap: mapWithBothSA,
		},
		{
			desc:    "denied",
			ksa:     testKSA,
			hmsResp: testHMSRespDenied,
			hmsOK:   true,
			initMap: mapHasOnlyOtherSA,
			wantReq: wantHMSReq,
			wantMap: mapHasOnlyOtherSA,
		},
		{
			desc:     "previous permission invalidated",
			ksa:      testKSA,
			hmsResp:  testHMSRespDenied,
			hmsOK:    true,
			initMap:  mapWithBothSA,
			wantReq:  wantHMSReq,
			wantMap:  mapHasOnlyOtherSA,
			wantSync: true,
		},
		{
			desc:    "sa indexer get error",
			ksa:     testKSA,
			ksaErr:  fmt.Errorf("indexer error on get"),
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
			desc:           "verified ksa got removed",
			ksaKeyOverride: testSAKey,
			initMap:        mapWithBothSA,
			wantMap:        mapHasOnlyOtherSA,
			wantSync:       true,
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
			wantReq: wantHMSReq,
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
						KNSName:  testSA.Namespace,
						KSAName:  testSA.Name,
						GSAEmail: "denied_gsa",
					},
				},
			},
			hmsOK:   true,
			initMap: mapWithBothSA,
			wantReq: &authorizeSAMappingRequest{
				RequestedMappings: []serviceAccountMapping{
					{
						KNSName:  testSA.Namespace,
						KSAName:  testSA.Name,
						GSAEmail: "denied_gsa",
					},
				},
			},
			wantMap:  mapHasOnlyOtherSA,
			wantSync: true,
		},
		{
			desc: "previous permission replaced by new annotation",
			ksa:  newKSA(testSA, "new_permitted_gsa"),
			hmsResp: &authorizeSAMappingResponse{
				PermittedMappings: []serviceAccountMapping{
					{
						KNSName:  testSA.Namespace,
						KSAName:  testSA.Name,
						GSAEmail: "new_permitted_gsa",
					},
				},
			},
			hmsOK:   true,
			initMap: mapHasOnlyTestSA,
			wantReq: &authorizeSAMappingRequest{
				RequestedMappings: []serviceAccountMapping{
					{
						KNSName:  testSA.Namespace,
						KSAName:  testSA.Name,
						GSAEmail: "new_permitted_gsa",
					},
				},
			},
			wantMap: map[serviceAccount]gsaEmail{
				testSA: gsaEmail("new_permitted_gsa"),
			},
			wantSync: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
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
				saIndexer:   fakeIndexer{obj: tc.ksa, err: tc.ksaErr},
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
			gotRaw := hmsServer.getLastRequest()
			if gotRaw == nil && tc.wantReq != nil {
				t.Errorf("expecting HMS request %v but it was not made", tc.wantReq)
			}
			if gotRaw != nil {
				if tc.wantReq == nil {
					t.Errorf("got HMS request %v but one was not expected", gotRaw)
				} else {
					var gotReq authorizeSAMappingRequest
					if err := json.Unmarshal(gotRaw, &gotReq); err != nil {
						t.Errorf("got invalid HMS request %v: %v", gotRaw, err)
					}
					if !reflect.DeepEqual(gotReq, *tc.wantReq) {
						t.Errorf("got request %v; want %v", gotReq, tc.wantReq)
					}
				}
			}
		})
	}
}

func newCM() *core.ConfigMap {
	return &core.ConfigMap{
		TypeMeta: meta.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: meta.ObjectMeta{
			Name:      verifiedSAConfigMapName,
			Namespace: verifiedSAConfigMapNamespace,
		},
		BinaryData: make(map[string][]byte),
	}
}

func newCMFromSAMap(t *testing.T, saMap *map[serviceAccount]gsaEmail) *core.ConfigMap {
	cm := newCM()
	if saMap != nil {
		text, err := json.Marshal(saMap)
		if err != nil {
			t.Fatalf("Failed to encode %v: %v", saMap, err)
		}
		cm.BinaryData[verifiedSAConfigMapKey] = text
	}
	return cm
}

func newCMFromData(data []byte) *core.ConfigMap {
	cm := newCM()
	cm.BinaryData[verifiedSAConfigMapKey] = data
	return cm
}

func TestConfigMapPersist(t *testing.T) {
	testSA := serviceAccount{"foo", "bar"}
	testGSA := gsaEmail("bar@testproject.com")
	otherSA := serviceAccount{"otherNamespace", "otherName"}
	otherGSA := gsaEmail("othergsa@testproject.com")
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
	mapHasZeroSA := map[serviceAccount]gsaEmail{}

	cmRes := schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}
	wantCMKey := fmt.Sprintf("%s/%s", verifiedSAConfigMapNamespace, verifiedSAConfigMapName)

	tests := []struct {
		desc        string
		indexerObj  interface{}
		indexerErr  error
		keyOverride string
		testSAMap   map[serviceAccount]gsaEmail
		failUpdate  bool
		wantActions []ktesting.Action
		wantErr     bool
	}{
		{
			desc:       "update to add SA",
			indexerObj: newCMFromSAMap(t, &mapHasOnlyOtherSA),
			testSAMap:  mapWithBothSA,
			wantActions: []ktesting.Action{
				ktesting.NewUpdateAction(cmRes, verifiedSAConfigMapNamespace, newCMFromSAMap(t, &mapWithBothSA)),
			},
		},
		{
			desc:        "create",
			keyOverride: wantCMKey,
			testSAMap:   mapWithBothSA,
			wantActions: []ktesting.Action{
				ktesting.NewCreateAction(cmRes, verifiedSAConfigMapNamespace, newCMFromSAMap(t, &mapWithBothSA)),
			},
		},
		{
			desc:        "no update necessary",
			indexerObj:  newCMFromSAMap(t, &mapWithBothSA),
			testSAMap:   mapWithBothSA,
			wantActions: []ktesting.Action{},
		},
		{
			desc:       "update to removing one SA",
			indexerObj: newCMFromSAMap(t, &mapWithBothSA),
			testSAMap:  mapHasOnlyTestSA,
			wantActions: []ktesting.Action{
				ktesting.NewUpdateAction(cmRes, verifiedSAConfigMapNamespace, newCMFromSAMap(t, &mapHasOnlyTestSA)),
			},
		},
		{
			desc:       "update to remove the last SA",
			indexerObj: newCMFromSAMap(t, &mapHasOnlyTestSA),
			testSAMap:  mapHasZeroSA,
			wantActions: []ktesting.Action{
				ktesting.NewUpdateAction(cmRes, verifiedSAConfigMapNamespace, newCMFromSAMap(t, &mapHasZeroSA)),
			},
		},
		{
			desc:        "ignore invalid configmap key",
			keyOverride: "invalid key",
			wantActions: []ktesting.Action{},
		},
		{
			desc:        "ignore other configmap key",
			keyOverride: "something/else",
			wantActions: []ktesting.Action{},
		},
		{
			desc:        "retry on indexer error",
			keyOverride: "dont/care",
			indexerErr:  fmt.Errorf("indexer get failure"),
			wantActions: []ktesting.Action{},
		},
		{
			desc:       "delete after api failure",
			indexerObj: newCMFromSAMap(t, &mapHasOnlyOtherSA),
			failUpdate: true,
			testSAMap:  mapWithBothSA,
			wantErr:    true,
			wantActions: []ktesting.Action{
				ktesting.NewUpdateAction(cmRes, verifiedSAConfigMapNamespace, newCMFromSAMap(t, &mapWithBothSA)),
				ktesting.NewDeleteAction(cmRes, verifiedSAConfigMapNamespace, wantCMKey),
			},
		},
		{
			desc:        "correct serialization",
			keyOverride: wantCMKey,
			testSAMap: map[serviceAccount]gsaEmail{
				testSA: testGSA,
			},
			wantActions: []ktesting.Action{
				ktesting.NewCreateAction(cmRes, verifiedSAConfigMapNamespace,
					newCMFromData([]byte(`{"foo/bar":"bar@testproject.com"}`))),
			},
		},
		{
			desc:        "correct serialization for empty namespace",
			keyOverride: wantCMKey,
			testSAMap: map[serviceAccount]gsaEmail{
				serviceAccount{"", "namespaceLess"}: gsaEmail("yetanothergsa@random.com"),
			},
			wantActions: []ktesting.Action{
				ktesting.NewCreateAction(cmRes, verifiedSAConfigMapNamespace,
					newCMFromData([]byte(`{"default/namespaceLess":"yetanothergsa@random.com"}`))),
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			c := fake.NewSimpleClientset()
			if tc.indexerObj != nil {
				c.Tracker().Add(tc.indexerObj.(*core.ConfigMap))
			}
			if tc.failUpdate {
				c.PrependReactor("update", "configmaps", func(action ktesting.Action) (bool, runtime.Object, error) {
					return true, nil, errors.NewInternalError(fmt.Errorf("fake internal error"))
				})
			}

			m := newSAMap()
			for sa, gsa := range tc.testSAMap {
				m.add(sa, gsa)
			}
			sav := &serviceAccountVerifier{
				c:           c,
				cmIndexer:   fakeIndexer{obj: tc.indexerObj, err: tc.indexerErr},
				verifiedSAs: m,
			}

			var key string
			if tc.keyOverride != "" {
				key = tc.keyOverride
			} else {
				if tc.indexerObj == nil {
					t.Fatalf("testcase error: keyOverride must be set if indexerObj is nil")
				}
				var err error
				key, err = cache.MetaNamespaceKeyFunc(tc.indexerObj)
				if err != nil {
					t.Fatalf("error getting key for CM %+v: %v", tc.indexerObj, err)
				}
			}

			err := sav.persist(key)

			if tc.wantErr && err == nil {
				t.Error("expecting but did not get an error")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("not expecting error but got %v", err)
			}
			if !reflect.DeepEqual(tc.wantActions, c.Actions()) {
				t.Errorf("got actions:\n%+v\nwant actions\n%+v", c.Actions(), tc.wantActions)
			}
		})
	}
}
