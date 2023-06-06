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
package pods

import (
	"context"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/cloud-provider-gcp/cmd/gcp-controller-manager/dpwi/serviceaccounts"
)

const (
	testNamespace = "testnamespace"
	testSAName    = "testsa"
	testGSA       = "testsa@testnamespace.wonderland"
	testNode      = "gke-test-node"
)

type fakeVerifier struct {
}

func (v *fakeVerifier) VerifiedGSA(ctx context.Context, ksa serviceaccounts.ServiceAccount) (serviceaccounts.GSAEmail, error) {
	if ksa.Namespace == testNamespace && ksa.Name == testSAName {
		return testGSA, nil
	}
	return "", nil
}

type fakeSyncer struct {
	count int
}

func (s *fakeSyncer) EnqueueKey(_ string) {
	s.count++
}

type fakeIndexer struct {
	cache.Indexer
	obj interface{}
	err error
}

func (f fakeIndexer) GetByKey(key string) (interface{}, bool, error) {
	return f.obj, f.obj != nil, f.err
}

func TestProcess(t *testing.T) {
	tests := []struct {
		desc          string
		pod           *v1.Pod
		nodeName      string
		wantSyncCount int
	}{
		{
			desc: "no node name",
			pod:  newPod(testNamespace, testSAName, ""),
		},
		{
			desc:     "no ksa",
			pod:      newPod(testNamespace, "", testNode),
			nodeName: testNode,
		},
		{
			desc:     "no verified gsa",
			pod:      newPod(testNamespace, "ksa-without-gsa", testNode),
			nodeName: testNode,
		},
		{
			desc:          "sync",
			pod:           newPod(testNamespace, testSAName, testNode),
			nodeName:      testNode,
			wantSyncCount: 1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			syncer := &fakeSyncer{}
			handler := &Handler{
				podIndexer: &fakeIndexer{obj: tc.pod},
				verifier:   &fakeVerifier{},
				syncer:     syncer,
			}

			podKey, err := cache.MetaNamespaceKeyFunc(tc.pod)
			if err != nil {
				t.Errorf("Failed to get key for pod %v: %v", tc.pod, err)
			}
			err = handler.process(context.Background(), podKey)
			if err != nil {
				t.Fatalf("process() failed: %v", err)
			}

			if got, want := syncer.count, tc.wantSyncCount; got != want {
				t.Errorf("Got sync count %v; want %v", got, want)
			}
		})
	}
}

func newPod(namespace, sa, node string) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "testPod",
		},
		Spec: v1.PodSpec{
			ServiceAccountName: sa,
			NodeName:           node,
		},
	}
}
