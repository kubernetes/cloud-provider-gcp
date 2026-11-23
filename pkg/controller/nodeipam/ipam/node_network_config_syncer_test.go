/*
Copyright 2026 The Kubernetes Authors.

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

package ipam

import (
	"context"
	"testing"

	nncv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/nodenetworkconfig/v1"
	nncfake "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/clientset/versioned/fake"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
)

func TestNodeNetworkConfigSyncer_EnsureNodeNetworkConfig(t *testing.T) {
	nodeName := "test-node-1"
	testNode := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
			UID:  "test-node-uid-12345",
		},
	}

	tests := []struct {
		desc          string
		existingNodes []*v1.Node
		existingNNCs  []*nncv1.NodeNetworkConfig
		syncKey       string
		wantErr       bool
		verifyNNC     bool
		wantOwnerUID  string
	}{
		{
			desc:          "create new NodeNetworkConfig CR when node exists and CR does not exist",
			existingNodes: []*v1.Node{testNode},
			existingNNCs:  nil,
			syncKey:       nodeName,
			wantErr:       false,
			verifyNNC:     true,
			wantOwnerUID:  "test-node-uid-12345",
		},
		{
			desc:          "do nothing when NodeNetworkConfig CR already exists",
			existingNodes: []*v1.Node{testNode},
			existingNNCs: []*nncv1.NodeNetworkConfig{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: nodeName,
					},
				},
			},
			syncKey:      nodeName,
			wantErr:      false,
			verifyNNC:    true,
			wantOwnerUID: "",
		},
		{
			desc:          "do nothing when node does not exist in lister",
			existingNodes: nil,
			existingNNCs:  nil,
			syncKey:       "non-existent-node",
			wantErr:       false,
			verifyNNC:     false,
		},
		{
			desc:          "sync with namespaced key works correctly",
			existingNodes: []*v1.Node{testNode},
			existingNNCs:  nil,
			syncKey:       "default/" + nodeName,
			wantErr:       false,
			verifyNNC:     true,
			wantOwnerUID:  "test-node-uid-12345",
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			var k8sObjs []runtime.Object
			for _, n := range tc.existingNodes {
				k8sObjs = append(k8sObjs, n)
			}
			kubeClient := fake.NewSimpleClientset(k8sObjs...)
			informerFactory := informers.NewSharedInformerFactory(kubeClient, 0)
			nodeInformer := informerFactory.Core().V1().Nodes()
			for _, n := range tc.existingNodes {
				nodeInformer.Informer().GetStore().Add(n)
			}

			var nncObjs []runtime.Object
			for _, nnc := range tc.existingNNCs {
				nncObjs = append(nncObjs, nnc)
			}
			nncClient := nncfake.NewSimpleClientset(nncObjs...)

			syncer := NewNodeNetworkConfigSyncer(nncClient, nodeInformer.Lister())

			err := syncer.sync(tc.syncKey)
			if (err != nil) != tc.wantErr {
				t.Fatalf("sync(%q) error = %v, wantErr = %v", tc.syncKey, err, tc.wantErr)
			}

			if tc.verifyNNC {
				nnc, err := nncClient.NetworkingV1().NodeNetworkConfigs().Get(context.Background(), nodeName, metav1.GetOptions{})
				if err != nil {
					t.Fatalf("Failed to get NodeNetworkConfig CR: %v", err)
				}
				if nnc.Name != nodeName {
					t.Errorf("NodeNetworkConfig name = %s, want %s", nnc.Name, nodeName)
				}
				if tc.wantOwnerUID != "" {
					if len(nnc.OwnerReferences) == 0 || string(nnc.OwnerReferences[0].UID) != tc.wantOwnerUID {
						t.Errorf("NodeNetworkConfig owner reference UID mismatch, got %#v", nnc.OwnerReferences)
					}
					if nnc.OwnerReferences[0].Kind != "Node" {
						t.Errorf("NodeNetworkConfig owner reference Kind = %s, want Node", nnc.OwnerReferences[0].Kind)
					}
				}
			}
		})
	}
}
