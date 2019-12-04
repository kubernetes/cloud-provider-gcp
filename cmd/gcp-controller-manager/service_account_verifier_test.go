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
	lister "k8s.io/client-go/listers/core/v1"
	ktesting "k8s.io/client-go/testing"
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
		ksa            *core.ServiceAccount
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

type fakeCMLister struct {
	lister.ConfigMapLister
	cm  *core.ConfigMap
	err error
}

func (f fakeCMLister) ConfigMaps(namespace string) lister.ConfigMapNamespaceLister {
	return &f
}

func (f fakeCMLister) Get(name string) (*core.ConfigMap, error) {
	return f.cm, f.err
}

func newCM(t *testing.T, saMap *map[serviceAccount]gsaEmail) *core.ConfigMap {
	cm := &core.ConfigMap{
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
	if saMap != nil {
		text, err := json.Marshal(saMap)
		if err != nil {
			t.Fatalf("Failed to encode %v: %v", saMap, err)
		}
		cm.BinaryData[verifiedSAConfigMapKey] = text
	}
	return cm
}

func TestConfigMapPersist(t *testing.T) {
	testSA := serviceAccount{"foo", "bar"}
	testGSA := gsaEmail("bar@testproject.iam.gserviceaccount.com")
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
	mapHasZeroSA := map[serviceAccount]gsaEmail{}

	cmRes := schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}
	wantCMKey := fmt.Sprintf("%s/%s", verifiedSAConfigMapNamespace, verifiedSAConfigMapName)

	tests := []struct {
		desc        string
		listerCM    *core.ConfigMap
		listerErr   error
		keyOverride string
		testSAMap   map[serviceAccount]gsaEmail
		failUpdate  bool
		wantActions []ktesting.Action
		wantErr     bool
	}{
		{
			desc:      "update to add SA",
			listerCM:  newCM(t, &mapHasOnlyOtherSA),
			testSAMap: mapWithBothSA,
			wantActions: []ktesting.Action{
				ktesting.NewUpdateAction(cmRes, verifiedSAConfigMapNamespace, newCM(t, &mapWithBothSA)),
			},
		},
		{
			desc:        "create",
			listerErr:   errors.NewNotFound(schema.GroupResource{}, "configmap"),
			keyOverride: wantCMKey,
			testSAMap:   mapWithBothSA,
			wantActions: []ktesting.Action{
				ktesting.NewCreateAction(cmRes, verifiedSAConfigMapNamespace, newCM(t, &mapWithBothSA)),
			},
		},
		{
			desc:        "no update necessary",
			listerCM:    newCM(t, &mapWithBothSA),
			testSAMap:   mapWithBothSA,
			wantActions: []ktesting.Action{},
		},
		{
			desc:      "update to removing one SA",
			listerCM:  newCM(t, &mapWithBothSA),
			testSAMap: mapHasOnlyTestSA,
			wantActions: []ktesting.Action{
				ktesting.NewUpdateAction(cmRes, verifiedSAConfigMapNamespace, newCM(t, &mapHasOnlyTestSA)),
			},
		},
		{
			desc:      "update to remove the last SA",
			listerCM:  newCM(t, &mapHasOnlyTestSA),
			testSAMap: mapHasZeroSA,
			wantActions: []ktesting.Action{
				ktesting.NewUpdateAction(cmRes, verifiedSAConfigMapNamespace, newCM(t, &mapHasZeroSA)),
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
			desc:       "delete after api failure",
			listerCM:   newCM(t, &mapHasOnlyOtherSA),
			failUpdate: true,
			testSAMap:  mapWithBothSA,
			wantErr:    true,
			wantActions: []ktesting.Action{
				ktesting.NewUpdateAction(cmRes, verifiedSAConfigMapNamespace, newCM(t, &mapWithBothSA)),
				ktesting.NewDeleteAction(cmRes, verifiedSAConfigMapNamespace, wantCMKey),
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			c := fake.NewSimpleClientset()
			if tc.listerCM != nil {
				c.Tracker().Add(tc.listerCM)
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
				cmls:        fakeCMLister{cm: tc.listerCM, err: tc.listerErr},
				verifiedSAs: m,
			}

			var key string
			if tc.keyOverride != "" {
				key = tc.keyOverride
			} else {
				if tc.listerCM == nil {
					t.Fatalf("testcase error: keyOverride must be set if listerCM is nil")
				}
				var err error
				key, err = cache.MetaNamespaceKeyFunc(tc.listerCM)
				if err != nil {
					t.Fatalf("error getting key for CM %+v: %v", tc.listerCM, err)
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
