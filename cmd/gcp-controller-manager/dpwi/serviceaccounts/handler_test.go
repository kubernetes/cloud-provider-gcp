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
package serviceaccounts

import (
	"context"
	"reflect"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

const (
	testNode1 = "gke-test-node-1"
	testNode2 = "gke-test-node-2"
	testNode3 = "gke-test-node-3"
)

var (
	pod1 = newPod("pod1", ksa1, testNode1)
	pod2 = newPod("pod2", ksa2, testNode2)
	pod3 = newPod("pod3", ksa3, testNode3)
)

type fakePodIndexer struct {
	cache.Indexer
	pods []*v1.Pod
	node *v1.Node
}

func (f *fakePodIndexer) GetByKey(key string) (interface{}, bool, error) {
	return f.node, true, nil
}
func (f *fakePodIndexer) ByIndex(indexName, indexedValue string) ([]interface{}, error) {
	if indexName != indexByKSA {
		return nil, nil
	}
	var res []interface{}
	for _, p := range f.pods {
		ksa := ServiceAccount{Namespace: p.Namespace, Name: p.Spec.ServiceAccountName}
		if ksa.Key() == indexedValue {
			res = append(res, p)
		}
	}
	return res, nil
}

type counter struct {
	notifyCMCount     int
	notifySyncerCount map[string]int
}

func (c *counter) notifyConfigmapHandler() {
	c.notifyCMCount++
}
func (c *counter) notifyNodeSyncer(node string) {
	c.notifySyncerCount[node]++
}

func TestProcess(t *testing.T) {
	tests := []struct {
		desc                  string
		ksas                  []ServiceAccount
		wantNotifyCMCount     int
		wantNotifySyncerCount map[string]int
	}{
		{
			desc:                  "no permitted GSA",
			ksas:                  []ServiceAccount{ksa3},
			wantNotifySyncerCount: map[string]int{},
		},
		{
			desc:              "one KSA on one node",
			ksas:              []ServiceAccount{ksa1},
			wantNotifyCMCount: 1,
			wantNotifySyncerCount: map[string]int{
				testNode1: 1,
			},
		},
		{
			desc:              "multipe KSAs on multiple nodes",
			ksas:              []ServiceAccount{ksa1, ksa2, ksa1},
			wantNotifyCMCount: 2,
			wantNotifySyncerCount: map[string]int{
				testNode1: 1,
				testNode2: 1,
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			counter := counter{notifySyncerCount: make(map[string]int)}
			v, _ := setUpDefaultVerifierForTest(t)
			h := &Handler{
				podIndexer: &fakePodIndexer{
					pods: []*v1.Pod{pod1, pod2, pod3},
				},
				verifier:               v,
				notifyConfigmapHandler: counter.notifyConfigmapHandler,
				notifyNodeSyncer:       counter.notifyNodeSyncer,
			}
			for _, ksa := range tc.ksas {
				err := h.process(context.Background(), ksa.Key())
				if err != nil {
					t.Errorf("process(%v) failed: %v", ksa, err)
				}
			}
			if got, want := counter.notifyCMCount, tc.wantNotifyCMCount; got != want {
				t.Errorf("notifyCMCount=%d, want %d", got, want)
			}
			if got, want := counter.notifySyncerCount, tc.wantNotifySyncerCount; !reflect.DeepEqual(got, want) {
				t.Errorf("notifySyncerCount=%v, want %v", got, want)
			}
		})
	}
}

func newPod(name string, ksa ServiceAccount, node string) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ksa.Namespace,
			Name:      name,
		},
		Spec: v1.PodSpec{
			ServiceAccountName: ksa.Name,
			NodeName:           node,
		},
	}
}
