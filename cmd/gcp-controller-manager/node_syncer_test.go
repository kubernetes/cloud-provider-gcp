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
	"sort"
	"testing"
	"time"

	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	testNode      = "test-instance"
	testLoc       = "us-nowhere9-x"
	testNamespace = "test-namespace"
)

type fakeIndexer struct {
	cache.Indexer
	obj interface{}
	err error
}

func (f fakeIndexer) GetByKey(key string) (interface{}, bool, error) {
	return f.obj, f.obj != nil, f.err
}

type fakeQueue struct {
	workqueue.DelayingInterface
	q []types.UID
}

func (f *fakeQueue) AddAfter(item interface{}, duration time.Duration) {
	f.q = append(f.q, item.(types.UID))
}

func newPod(uid types.UID, key, ksa string) *core.Pod {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		panic(err)
	}
	return &core.Pod{
		TypeMeta: meta.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: meta.ObjectMeta{
			Namespace: namespace,
			Name:      name,
			UID:       uid,
		},
		Spec: core.PodSpec{
			NodeName:           testNode,
			ServiceAccountName: ksa,
		},
	}
}

func TestNodeSync(t *testing.T) {
	verifiedSAs := saMap{
		ma: map[serviceAccount]gsaEmail{
			{testNamespace, "testKSA0"}: "testGSA0",
			{testNamespace, "testKSA1"}: "testGSA1",
			{testNamespace, "testKSA2"}: "testGSA2",
		},
	}
	podKey0 := testNamespace + "/testPod0"
	podKey1 := testNamespace + "/testPod1"
	podKey2 := testNamespace + "/testPod2"
	podKeyUnauthz := testNamespace + "/testPodUnauthz"
	podUID0 := types.UID("testUIDForTestPod0")
	podUID1 := types.UID("testUIDForTestPod1")
	podUID2 := types.UID("testUIDForTestPod2")
	node := &core.Node{
		ObjectMeta: meta.ObjectMeta{
			Name: testNode,
		},
		Spec: core.NodeSpec{
			ProviderID: fmt.Sprintf("gce://project/%s/instance", testLoc),
		},
	}

	tests := []struct {
		desc        string
		idxObj      interface{}
		idxErr      error
		keyOverride string
		initPodMap  podMap
		delayGSARm  bool
		wantErr     bool
		wantPodRmQ  []types.UID
		wantPodMap  podMap
		wantGSASync []gsaEmail
	}{
		{
			desc:        "add first pod with authorized gsa",
			idxObj:      newPod(podUID0, podKey0, "testKSA0"),
			wantPodMap:  map[types.UID]pod{podUID0: pod{podKey0, "testGSA0"}},
			wantGSASync: []gsaEmail{"testGSA0"},
			wantPodRmQ:  []types.UID{},
		},
		{
			desc:       "add pod with unauthorized gsa",
			initPodMap: map[types.UID]pod{podUID0: pod{podKey0, "testGSA0"}},
			idxObj:     newPod(podUID1, podKeyUnauthz, "testKSAUnauthz"),
			wantPodMap: map[types.UID]pod{podUID0: pod{podKey0, "testGSA0"}},
			wantPodRmQ: []types.UID{},
		},
		{
			desc:        "add pod with a new gsa",
			initPodMap:  map[types.UID]pod{podUID0: pod{podKey0, "testGSA0"}, podUID1: pod{podKey1, "testGSA1"}},
			idxObj:      newPod(podUID2, podKey2, "testKSA2"),
			wantPodMap:  map[types.UID]pod{podUID0: pod{podKey0, "testGSA0"}, podUID1: pod{podKey1, "testGSA1"}, podUID2: pod{podKey2, "testGSA2"}},
			wantGSASync: []gsaEmail{"testGSA0", "testGSA1", "testGSA2"},
			wantPodRmQ:  []types.UID{},
		},
		{
			desc:        "update pod with different gsa",
			initPodMap:  map[types.UID]pod{podUID0: pod{podKey0, "testGSA1"}},
			idxObj:      newPod(podUID0, podKey0, "testKSA2"),
			wantPodMap:  map[types.UID]pod{podUID0: pod{podKey0, "testGSA2"}},
			wantGSASync: []gsaEmail{"testGSA2"},
			wantPodRmQ:  []types.UID{},
		},
		{
			desc:        "add pod with repeating gsa",
			initPodMap:  map[types.UID]pod{podUID0: pod{podKey0, "testGSA1"}},
			idxObj:      newPod(podUID1, podKey1, "testKSA1"),
			wantPodMap:  map[types.UID]pod{podUID0: pod{podKey0, "testGSA1"}, podUID1: pod{podKey1, "testGSA1"}},
			wantGSASync: []gsaEmail{"testGSA1"},
			wantPodRmQ:  []types.UID{},
		},
		{
			desc:        "get pod failed",
			idxErr:      fmt.Errorf("indexer error on pod get"),
			keyOverride: podKey0,
			wantErr:     true,
			wantPodRmQ:  []types.UID{},
		},
		{
			desc:        "pod delete with unique gsa",
			initPodMap:  map[types.UID]pod{podUID0: pod{podKey0, "testGSA0"}, podUID1: pod{podKey1, "testGSA1"}},
			keyOverride: podKey0,
			wantPodMap:  map[types.UID]pod{podUID1: pod{podKey1, "testGSA1"}},
			wantGSASync: []gsaEmail{"testGSA1"},
			wantPodRmQ:  []types.UID{},
		},
		{
			desc:        "pod delete with repeating gsa",
			initPodMap:  map[types.UID]pod{podUID0: pod{podKey0, "testGSA0"}, podUID1: pod{podKey1, "testGSA0"}},
			keyOverride: podKey0,
			wantPodMap:  map[types.UID]pod{podUID1: pod{podKey1, "testGSA0"}},
			wantGSASync: []gsaEmail{"testGSA0"},
			wantPodRmQ:  []types.UID{},
		},
		{
			desc:        "delete pods with the same key and the same gsa",
			initPodMap:  map[types.UID]pod{podUID0: pod{podKey0, "testGSA0"}, podUID1: pod{podKey0, "testGSA0"}, podUID2: pod{podKey2, "testGSA2"}},
			keyOverride: podKey0,
			wantPodMap:  map[types.UID]pod{podUID2: pod{podKey2, "testGSA2"}},
			wantGSASync: []gsaEmail{"testGSA2"},
			wantPodRmQ:  []types.UID{},
		},
		{
			desc:        "delete pods with the same key and but different gsa",
			initPodMap:  map[types.UID]pod{podUID0: pod{podKey0, "testGSA0"}, podUID1: pod{podKey0, "testGSA1"}, podUID2: pod{podKey2, "testGSA2"}},
			keyOverride: podKey0,
			wantPodMap:  map[types.UID]pod{podUID2: pod{podKey2, "testGSA2"}},
			wantGSASync: []gsaEmail{"testGSA2"},
			wantPodRmQ:  []types.UID{},
		},
		{
			desc:        "pod delete with unique gsa with delay",
			initPodMap:  map[types.UID]pod{podUID0: pod{podKey0, "testGSA0"}, podUID1: pod{podKey1, "testGSA1"}},
			keyOverride: podKey0,
			wantPodMap:  map[types.UID]pod{podUID1: pod{podKey1, "testGSA1"}},
			wantGSASync: []gsaEmail{"testGSA1"},
			delayGSARm:  true,
			wantPodRmQ:  []types.UID{podUID0},
		},
		{
			desc:        "pod delete with repeating gsa with delay",
			initPodMap:  map[types.UID]pod{podUID0: pod{podKey0, "testGSA0"}, podUID1: pod{podKey1, "testGSA0"}},
			keyOverride: podKey0,
			wantPodMap:  map[types.UID]pod{podUID1: pod{podKey1, "testGSA0"}},
			wantGSASync: []gsaEmail{"testGSA0"},
			delayGSARm:  true,
			wantPodRmQ:  []types.UID{podUID0},
		},
		{
			desc:        "delete pods with the same key and the same gsa with delay",
			initPodMap:  map[types.UID]pod{podUID0: pod{podKey0, "testGSA0"}, podUID1: pod{podKey0, "testGSA0"}, podUID2: pod{podKey2, "testGSA2"}},
			keyOverride: podKey0,
			wantPodMap:  map[types.UID]pod{podUID2: pod{podKey2, "testGSA2"}},
			wantGSASync: []gsaEmail{"testGSA2"},
			delayGSARm:  true,
			wantPodRmQ:  []types.UID{podUID0, podUID1},
		},
		{
			desc:        "delete pods with the same key and but different gsa with delay",
			initPodMap:  map[types.UID]pod{podUID0: pod{podKey0, "testGSA0"}, podUID1: pod{podKey0, "testGSA1"}, podUID2: pod{podKey2, "testGSA2"}},
			keyOverride: podKey0,
			wantPodMap:  map[types.UID]pod{podUID2: pod{podKey2, "testGSA2"}},
			wantGSASync: []gsaEmail{"testGSA2"},
			delayGSARm:  true,
			wantPodRmQ:  []types.UID{podUID0, podUID1},
		},
	}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			hmsServer := newTestHMS(nil, true)
			hmsClient, err := newHMSClient(hmsServer.server.URL, nil)
			if err != nil {
				t.Fatalf("error creating test HMS client: %v", err)
			}
			nodes := newNodeMap()
			for podUID, pod := range tc.initPodMap {
				nodes.add(testNode, podUID, pod.key, pod.gsa)
			}
			pq := fakeQueue{
				q: []types.UID{},
			}
			ns := &nodeSyncer{
				indexer:        fakeIndexer{obj: tc.idxObj, err: tc.idxErr},
				hms:            hmsClient,
				verifiedSAs:    &verifiedSAs,
				nodes:          nodes,
				zones:          newNodeZones(fake.NewSimpleClientset(node)),
				delayGSARemove: tc.delayGSARm,
				podRemoveQueue: &pq,
			}

			podKey := tc.keyOverride
			if podKey == "" {
				podKey, err = cache.MetaNamespaceKeyFunc(tc.idxObj)
				if err != nil {
					t.Errorf("failed to get key for obj %v: %v", tc.idxObj, err)
				}
			}

			err = ns.process(podKey)

			if tc.wantErr && err == nil {
				t.Error("expecting but did not get an error")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("not expecting error but got %v", err)
			}
			sort.Slice(tc.wantPodRmQ, func(i, j int) bool {
				return string(tc.wantPodRmQ[i]) > string(tc.wantPodRmQ[j])
			})
			sort.Slice(pq.q, func(i, j int) bool {
				return string(pq.q[i]) > string(pq.q[j])
			})
			if want, got := tc.wantPodRmQ, pq.q; !reflect.DeepEqual(want, got) {
				t.Errorf("delayed pod remove queue mismatch: %v\nwant: %v", got, want)
			}

			for _, uid := range pq.q {
				if err := ns.removePod(uid); err != nil {
					t.Errorf("not expecting removePod error %v", err)
				}
			}

			if got, want := ns.nodes.m[testNode], tc.wantPodMap; !reflect.DeepEqual(got, want) {
				t.Errorf("pod map mistmatch: %v\nwant: %v", got, want)
			}
			gotRaw := hmsServer.getLastRequest()
			if gotRaw == nil && tc.wantGSASync != nil {
				t.Errorf("expecting %v to be synced but did not get request", tc.wantGSASync)
			}
			if gotRaw != nil {
				if tc.wantGSASync == nil {
					t.Errorf("got sync request %v but it was not expected", gotRaw)
				} else {
					var gotReq syncNodeRequest
					if err := json.Unmarshal(gotRaw, &gotReq); err != nil {
						t.Errorf("got invalid sync request %v: %v", gotRaw, err)
					} else {
						wantReq := syncNodeRequest{
							NodeName:  testNode,
							NodeZone:  testLoc,
							GSAEmails: make([]string, len(tc.wantGSASync)),
						}
						for i := range tc.wantGSASync {
							wantReq.GSAEmails[i] = string(tc.wantGSASync[i])
						}
						sort.Strings(wantReq.GSAEmails)
						sort.Strings(gotReq.GSAEmails)
						if !reflect.DeepEqual(gotReq, wantReq) {
							t.Errorf("got sync request %v; want %v", gotReq, wantReq)
						}
					}
				}
			}
		})
	}
}
