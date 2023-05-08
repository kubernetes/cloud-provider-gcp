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

package nodesyncer

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/cloud-provider-gcp/cmd/gcp-controller-manager/dpwi/hms"
	"k8s.io/cloud-provider-gcp/cmd/gcp-controller-manager/dpwi/serviceaccounts"
)

const (
	testNamespace = "testnamespace"
	testNode      = "gke-test-node"
	testZone      = "us-nowhere9-x"
)

var (
	ksa1         = serviceaccounts.ServiceAccount{Namespace: testNamespace, Name: "ksa1"}
	ksa2         = serviceaccounts.ServiceAccount{Namespace: testNamespace, Name: "ksa2"}
	ksa3         = serviceaccounts.ServiceAccount{Namespace: testNamespace, Name: "ksa3"}
	gsa1         = "gsa1@something.iam.gserviceaccount.com"
	gsa2         = "gsa2@something.iam.gserviceaccount.com"
	pod1         = newPod("pod1", ksa1, testNode)
	pod2         = newPod("pod2", ksa2, testNode)
	pod3         = newPod("pod3", ksa3, testNode)
	nodeWithZone = newNode(testZone)
)

type fakeVerifier struct {
	m map[serviceaccounts.ServiceAccount]serviceaccounts.GSAEmail
}

func (v *fakeVerifier) VerifiedGSA(ctx context.Context, ksa serviceaccounts.ServiceAccount) (serviceaccounts.GSAEmail, error) {
	val, ok := v.m[ksa]
	if ok {
		return val, nil
	}
	return "", nil
}

type fakeIndexer struct {
	cache.Indexer
	pods []*v1.Pod
	node *v1.Node
}

func (f *fakeIndexer) GetByKey(key string) (interface{}, bool, error) {
	return f.node, true, nil
}
func (f *fakeIndexer) ByIndex(indexName, indexedValue string) ([]interface{}, error) {
	if indexName != indexByNode {
		return nil, nil
	}
	var res []interface{}
	for _, p := range f.pods {
		res = append(res, p)
	}
	return res, nil
}
func TestProcess(t *testing.T) {
	tests := []struct {
		desc          string
		node          *v1.Node
		pods          []*v1.Pod
		wantErr       bool
		wantSyncCount int
		wantGSAs      []string
	}{
		{
			desc:    "zone is empty",
			node:    newNode(""),
			wantErr: true,
		},
		{
			desc: "no pod on node",
			node: nodeWithZone,
		},
		{
			desc: "no GSA on node",
			node: nodeWithZone,
			pods: []*v1.Pod{pod3},
		},
		{
			desc:          "one GSA on node",
			node:          nodeWithZone,
			pods:          []*v1.Pod{pod1},
			wantSyncCount: 1,
			wantGSAs:      []string{gsa1},
		},
		{
			desc:          "multiple GSAs on node",
			node:          nodeWithZone,
			pods:          []*v1.Pod{pod1, pod2, pod3},
			wantSyncCount: 1,
			wantGSAs:      []string{gsa1, gsa2},
		},
	}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			server := hms.NewFakeServer(map[string]string{
				ksa1.Key(): gsa1,
				ksa2.Key(): gsa2,
			})
			hms, err := hms.NewClient(server.Server.URL, nil)
			if err != nil {
				t.Fatalf("Failed to create HMS client")
			}
			h := &NodeHandler{
				podIndexer: &fakeIndexer{
					pods: tc.pods,
				},
				verifier: &fakeVerifier{
					m: map[serviceaccounts.ServiceAccount]serviceaccounts.GSAEmail{
						ksa1: serviceaccounts.GSAEmail(gsa1),
						ksa2: serviceaccounts.GSAEmail(gsa2),
					},
				},
				hms:         hms,
				nodeIndexer: &fakeIndexer{node: tc.node},
				nodeMap:     &nodeMap{m: make(map[string]*nodeGSAs)},
			}
			err = h.process(context.Background(), tc.node.Name)
			if got, want := err != nil, tc.wantErr; got != want {
				t.Fatalf("process()=%v, want %v", err, want)
			}
			if err != nil {
				return
			}
			// processing again shortly is no-op.
			h.process(context.Background(), tc.node.Name)
			if got, want := server.SyncCount[tc.node.Name], tc.wantSyncCount; got != want {
				t.Errorf("SyncCount=%d, want %d", got, want)
			}
			if got, want := server.SyncGSAs[tc.node.Name], tc.wantGSAs; !cmp.Equal(got, want) {
				t.Errorf("SyncGSAs=%v, want %v", got, want)
			}
		})
	}
}

func newPod(name string, ksa serviceaccounts.ServiceAccount, node string) *v1.Pod {
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

func newNode(zone string) *v1.Node {
	return &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNode,
			Labels: map[string]string{
				v1.LabelTopologyZone: zone,
			},
		},
	}
}
