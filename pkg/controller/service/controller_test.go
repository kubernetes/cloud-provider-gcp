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

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/informers"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes/fake"
	testingcore "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"
	cloudprovider "k8s.io/cloud-provider"
	consistencyutil "k8s.io/cloud-provider-gcp/pkg/controller/util/consistency"
	"k8s.io/component-base/featuregate"
)

type fakeRVGetter struct {
	rv string
}

func (f *fakeRVGetter) LastStoreSyncResourceVersion() string {
	return f.rv
}

func (f *fakeRVGetter) setRV(rv string) {
	f.rv = rv
}

type fakeCloud struct {
	cloudprovider.Interface
	balancer *fakeBalancer
}

func (f *fakeCloud) ProviderName() string {
	return "fake"
}

func (f *fakeCloud) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	return f.balancer, true
}

type fakeBalancer struct {
	cloudprovider.LoadBalancer
}

func (b *fakeBalancer) GetLoadBalancer(ctx context.Context, clusterName string, service *v1.Service) (status *v1.LoadBalancerStatus, exists bool, err error) {
	return &v1.LoadBalancerStatus{}, true, nil
}

func (b *fakeBalancer) EnsureLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) (*v1.LoadBalancerStatus, error) {
	return &v1.LoadBalancerStatus{
		Ingress: []v1.LoadBalancerIngress{{IP: "1.2.3.4"}},
	}, nil
}

func (b *fakeBalancer) EnsureLoadBalancerDeleted(ctx context.Context, clusterName string, service *v1.Service) error {
	return nil
}

func (b *fakeBalancer) UpdateLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) error {
	return nil
}

type serviceTestFixture struct {
	controller       *Controller
	serviceGetter    *fakeRVGetter
	nodeGetter       *fakeRVGetter
	consistencyStore consistencyutil.ConsistencyStore
	serviceInformer  coreinformers.ServiceInformer
	kubeClient       *fake.Clientset
	service          *v1.Service
}

func setupServiceTestFixture() *serviceTestFixture {
	lbClass := "networking.gke.io/l4-regional-external-legacy"
	service := &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       "default",
			Name:            "test-service",
			UID:             types.UID("svc-uid-1"),
			ResourceVersion: "1",
		},
		Spec: v1.ServiceSpec{
			Type:              v1.ServiceTypeLoadBalancer,
			LoadBalancerClass: &lbClass,
		},
	}

	kubeClient := fake.NewSimpleClientset(service)
	kubeClient.PrependReactor("patch", "services", func(action testingcore.Action) (handled bool, ret runtime.Object, err error) {
		svc := service.DeepCopy()
		svc.ResourceVersion = "2"
		return true, svc, nil
	})

	informerFactory := informers.NewSharedInformerFactory(kubeClient, 0*time.Second)
	serviceInformer := informerFactory.Core().V1().Services()
	nodeInformer := informerFactory.Core().V1().Nodes()

	cloud := &fakeCloud{balancer: &fakeBalancer{}}
	controller, err := New(
		cloud,
		kubeClient,
		serviceInformer,
		nodeInformer,
		"test-cluster",
		featuregate.NewFeatureGate(),
	)
	if err != nil {
		panic(err)
	}
	controller.eventRecorder = record.NewFakeRecorder(100)

	serviceGetter := &fakeRVGetter{rv: "1"}
	nodeGetter := &fakeRVGetter{rv: "1"}
	consistencyStore := consistencyutil.NewConsistencyStore(map[schema.GroupResource]consistencyutil.LastSyncRVGetter{
		{Group: "", Resource: "services"}: serviceGetter,
		{Group: "", Resource: "nodes"}:    nodeGetter,
	})
	controller.consistencyStore = consistencyStore

	return &serviceTestFixture{
		controller:       controller,
		serviceGetter:    serviceGetter,
		nodeGetter:       nodeGetter,
		consistencyStore: consistencyStore,
		serviceInformer:  serviceInformer,
		kubeClient:       kubeClient,
		service:          service,
	}
}

func TestServiceController_ConsistencyStoreStallsUntilInformerCatchesUp(t *testing.T) {
	f := setupServiceTestFixture()
	ctx := context.Background()
	key := "default/test-service"
	namespacedName := types.NamespacedName{Namespace: "default", Name: "test-service"}

	// 1. Initially consistent
	require.NoError(t, f.consistencyStore.EnsureReady(namespacedName))

	// 2. Add finalizer which mutates service and tracks write
	err := f.controller.addFinalizer(f.service)
	require.NoError(t, err)

	// 3. Reconciler stalls because informer is still at RV "1"
	err = f.controller.syncService(ctx, key)
	assert.Error(t, err)
	assert.Error(t, f.consistencyStore.EnsureReady(namespacedName))

	// 4. Advance informer RV to catch up
	f.serviceGetter.setRV("2")
	assert.NoError(t, f.consistencyStore.EnsureReady(namespacedName))

	// 5. Populate service in informer store and reconcile proceeds
	f.service.ResourceVersion = "2"
	f.service.Finalizers = []string{"service.k8s.io/load-balancer-cleanup"}
	err = f.serviceInformer.Informer().GetStore().Add(f.service)
	require.NoError(t, err)

	err = f.controller.syncService(ctx, key)
	assert.NoError(t, err)
}
