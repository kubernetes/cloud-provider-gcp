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

package nodelifecycle

import (
	"context"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	cloudprovider "k8s.io/cloud-provider"
)

type mockInstances struct {
	cloudprovider.Instances
	existsCalled map[string]bool
}

func (m *mockInstances) InstanceExistsByProviderID(ctx context.Context, providerID string) (bool, error) {
	if m.existsCalled == nil {
		m.existsCalled = make(map[string]bool)
	}
	m.existsCalled[providerID] = true
	return true, nil
}

func (m *mockInstances) InstanceShutdownByProviderID(ctx context.Context, providerID string) (bool, error) {
	return false, nil
}

type mockCloud struct {
	cloudprovider.Interface
	instances *mockInstances
}

func (m *mockCloud) Instances() (cloudprovider.Instances, bool) {
	return m.instances, true
}

func (m *mockCloud) InstancesV2() (cloudprovider.InstancesV2, bool) {
	return nil, false
}

func TestMonitorNodes_FilterLabel(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()
	informerFactory := informers.NewSharedInformerFactory(fakeClient, 0)
	nodeInformer := informerFactory.Core().V1().Nodes()

	managedNode := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "managed-node"},
		Spec:       v1.NodeSpec{ProviderID: "gce://project/zone/managed-node"},
		Status:     v1.NodeStatus{Conditions: []v1.NodeCondition{{Type: v1.NodeReady, Status: v1.ConditionFalse}}},
	}
	unmanagedNode := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "unmanaged-node",
			Labels: map[string]string{GKEUnmanagedNodeLabelKey: GKEUnmanagedNodeLabelValue},
		},
		Spec:   v1.NodeSpec{ProviderID: "gce://project/zone/unmanaged-node"},
		Status: v1.NodeStatus{Conditions: []v1.NodeCondition{{Type: v1.NodeReady, Status: v1.ConditionFalse}}},
	}
	labeledManagedNode := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "labeled-managed-node",
			Labels: map[string]string{GKEUnmanagedNodeLabelKey: "false"},
		},
		Spec:   v1.NodeSpec{ProviderID: "gce://project/zone/labeled-managed-node"},
		Status: v1.NodeStatus{Conditions: []v1.NodeCondition{{Type: v1.NodeReady, Status: v1.ConditionFalse}}},
	}

	nodeInformer.Informer().GetStore().Add(managedNode)
	nodeInformer.Informer().GetStore().Add(unmanagedNode)
	nodeInformer.Informer().GetStore().Add(labeledManagedNode)

	mockInst := &mockInstances{existsCalled: make(map[string]bool)}
	mockCl := &mockCloud{instances: mockInst}

	c, err := NewCloudNodeLifecycleController(nodeInformer, fakeClient, mockCl, 5*time.Minute)
	if err != nil {
		t.Fatalf("failed to create controller: %v", err)
	}

	c.MonitorNodes(context.Background())

	if !mockInst.existsCalled[managedNode.Spec.ProviderID] {
		t.Errorf("expected InstanceExistsByProviderID to be called for managed node, but it wasn't")
	}
	if !mockInst.existsCalled[labeledManagedNode.Spec.ProviderID] {
		t.Errorf("expected InstanceExistsByProviderID to be called for labeled managed node (false), but it wasn't")
	}
	if mockInst.existsCalled[unmanagedNode.Spec.ProviderID] {
		t.Errorf("expected InstanceExistsByProviderID NOT to be called for unmanaged node, but it was")
	}
}
