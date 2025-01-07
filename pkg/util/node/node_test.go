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

package node

import (
	"testing"

	networkv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/network/v1"
	"github.com/google/go-cmp/cmp"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/cloud-provider-gcp/pkg/controller/testutil"
)

func TestPatchNodeMultiNetwork(t *testing.T) {
	testCases := []struct {
		desc            string
		fakeNodeHandler *testutil.FakeNodeHandler
		nodeChanges     func(node *v1.Node)
		wantNode        *v1.Node
		expectErr       bool
	}{
		{
			desc: "Update empty node",
			fakeNodeHandler: &testutil.FakeNodeHandler{
				Existing: []*v1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node0",
						},
					},
				},
				Clientset: fake.NewSimpleClientset(),
			},
			nodeChanges: func(node *v1.Node) {
				node.Annotations = map[string]string{
					networkv1.NorthInterfacesAnnotationKey: "[{\"network\":\"test\",\"ipAddress\":\"10.1.1.1\"}]",
					networkv1.MultiNetworkAnnotationKey:    "[{\"name\":\"test\",\"cidrs\":[\"172.11.1.0/32\"],\"scope\":\"host-local\"}]",
				}
				node.Status.Capacity = map[v1.ResourceName]resource.Quantity{
					"networking.gke.io.networks/test.IP": *resource.NewQuantity(1, resource.DecimalSI),
				}
			},
		},
		{
			desc: "Add to annotations",
			fakeNodeHandler: &testutil.FakeNodeHandler{
				Existing: []*v1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node0",
							Annotations: map[string]string{
								"test": "abc",
							},
						},
					},
				},
				Clientset: fake.NewSimpleClientset(),
			},
			nodeChanges: func(node *v1.Node) {
				node.Annotations[networkv1.NorthInterfacesAnnotationKey] = "[{\"network\":\"test\",\"ipAddress\":\"10.1.1.1\"}]"
				node.Annotations[networkv1.MultiNetworkAnnotationKey] = "[{\"name\":\"test\",\"cidrs\":[\"172.11.1.0/32\"],\"scope\":\"host-local\"}]"
				node.Status.Capacity = map[v1.ResourceName]resource.Quantity{
					"networking.gke.io.networks/test.IP": *resource.NewQuantity(1, resource.DecimalSI),
				}
			},
		},
		{
			desc: "Add to capacity",
			fakeNodeHandler: &testutil.FakeNodeHandler{
				Existing: []*v1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node0",
						},
						Status: v1.NodeStatus{
							Capacity: map[v1.ResourceName]resource.Quantity{
								"CPU": *resource.NewQuantity(10, resource.DecimalSI),
							},
						},
					},
				},
				Clientset: fake.NewSimpleClientset(),
			},
			nodeChanges: func(node *v1.Node) {
				node.Annotations = map[string]string{
					networkv1.NorthInterfacesAnnotationKey: "[{\"network\":\"test\",\"ipAddress\":\"10.1.1.1\"}]",
					networkv1.MultiNetworkAnnotationKey:    "[{\"name\":\"test\",\"cidrs\":[\"172.11.1.0/32\"],\"scope\":\"host-local\"}]",
				}
				node.Status.Capacity["networking.gke.io.networks/test.IP"] = *resource.NewQuantity(1, resource.DecimalSI)
			},
		},
		{
			desc: "[invalid] update node spec",
			fakeNodeHandler: &testutil.FakeNodeHandler{
				Existing: []*v1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node0",
						},
					},
				},
				Clientset: fake.NewSimpleClientset(),
			},
			nodeChanges: func(node *v1.Node) {
				node.Spec.PodCIDR = "1.1.1.0/24"
			},
			expectErr: true,
		},
		{
			desc: "remove capacity",
			fakeNodeHandler: &testutil.FakeNodeHandler{
				Existing: []*v1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "node0",
						},
						Status: v1.NodeStatus{
							Capacity: map[v1.ResourceName]resource.Quantity{
								"CPU":                                *resource.NewQuantity(10, resource.DecimalSI),
								"networking.gke.io.networks/test.IP": *resource.NewQuantity(1, resource.DecimalSI),
							},
						},
					},
				},
				Clientset: fake.NewSimpleClientset(),
			},
			nodeChanges: func(node *v1.Node) {
				delete(node.Status.Capacity, "networking.gke.io.networks/test.IP")
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			// setup
			newNode := tc.fakeNodeHandler.Existing[0].DeepCopy()
			tc.nodeChanges(newNode)

			// test
			err := PatchNodeMultiNetwork(tc.fakeNodeHandler, newNode)
			if err != nil {
				t.Fatalf("unexpected error %v", err)
			}
			gotNode := tc.fakeNodeHandler.GetUpdatedNodesCopy()[0]
			diff := cmp.Diff(newNode, gotNode)
			if diff != "" && !tc.expectErr {
				t.Fatalf("PatchNodeMultiNetwork() node not updated (-want +got) = %s", diff)
			} else if diff == "" && tc.expectErr {
				t.Fatalf("PatchNodeMultiNetwork() expected error but got pass")
			}
		})
	}
}
