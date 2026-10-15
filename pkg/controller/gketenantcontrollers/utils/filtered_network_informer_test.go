/*
Copyright 2026 The Kubernetes Authors.
*/

package utils

import (
	"context"
	"sync"
	"testing"
	"time"

	apinetworkv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/network/v1"
	networkfake "github.com/GoogleCloudPlatform/gke-networking-api/client/network/clientset/versioned/fake"
	networkinformers "github.com/GoogleCloudPlatform/gke-networking-api/client/network/informers/externalversions"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

func TestFilteredNetworkSharedInformerFactory_TenantIsolation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1. Create fake network clientset and parent factory
	fakeNetworking := networkfake.NewSimpleClientset()
	parentFactory := networkinformers.NewSharedInformerFactory(fakeNetworking, 0*time.Second)

	// 2. Wrap it with FilteredNetworkSharedInformerFactory for tenant-A
	labelKey := "tenancy.gke.io/provider-config"
	filteredFactory := NewFilteredNetworkSharedInformerFactory(parentFactory, labelKey, "tenant-A", false)

	// 3. Create Networks and GNPs for tenant-A and tenant-B
	netA := &apinetworkv1.Network{
		ObjectMeta: metav1.ObjectMeta{
			Name: "net-a",
			Labels: map[string]string{
				labelKey: "tenant-A",
			},
		},
	}
	gnpA := &apinetworkv1.GKENetworkParamSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "gnp-a",
			Labels: map[string]string{
				labelKey: "tenant-A",
			},
		},
	}

	netB := &apinetworkv1.Network{
		ObjectMeta: metav1.ObjectMeta{
			Name: "net-b",
			Labels: map[string]string{
				labelKey: "tenant-B",
			},
		},
	}
	gnpB := &apinetworkv1.GKENetworkParamSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "gnp-b",
			Labels: map[string]string{
				labelKey: "tenant-B",
			},
		},
	}

	_, err := fakeNetworking.NetworkingV1().Networks().Create(ctx, netA, metav1.CreateOptions{})
	assert.NoError(t, err)
	_, err = fakeNetworking.NetworkingV1().Networks().Create(ctx, netB, metav1.CreateOptions{})
	assert.NoError(t, err)
	_, err = fakeNetworking.NetworkingV1().GKENetworkParamSets().Create(ctx, gnpA, metav1.CreateOptions{})
	assert.NoError(t, err)
	_, err = fakeNetworking.NetworkingV1().GKENetworkParamSets().Create(ctx, gnpB, metav1.CreateOptions{})
	assert.NoError(t, err)

	// Start parent informer factory
	// Create informers/listers from filtered factory
	netInformer := filteredFactory.Networking().V1().Networks()
	gnpInformer := filteredFactory.Networking().V1().GKENetworkParamSets()

	// Retrieve list and check to trigger Informer() creation/registration
	_ = netInformer.Informer()
	_ = gnpInformer.Informer()

	// Start parent informer factory
	stopCh := make(chan struct{})
	defer close(stopCh)
	parentFactory.Start(stopCh)

	// Wait for cache sync
	parentFactory.WaitForCacheSync(stopCh)

	// Verify tenant-A gets its own resources
	netLister := netInformer.Lister()
	gnpLister := gnpInformer.Lister()

	// Retrieve list and check
	nets, err := netLister.List(labels.Everything())
	assert.NoError(t, err)
	assert.Len(t, nets, 1)
	assert.Equal(t, "net-a", nets[0].Name)

	gnps, err := gnpLister.List(labels.Everything())
	assert.NoError(t, err)
	assert.Len(t, gnps, 1)
	assert.Equal(t, "gnp-a", gnps[0].Name)

	// Verify Get works for own tenant
	gotNet, err := netLister.Get("net-a")
	assert.NoError(t, err)
	assert.NotNil(t, gotNet)

	gotGNP, err := gnpLister.Get("gnp-a")
	assert.NoError(t, err)
	assert.NotNil(t, gotGNP)

	// Verify Get fails (or returns not found/nil) for cross-tenant
	gotNetB, err := netLister.Get("net-b")
	assert.Error(t, err) // Should error or return not found since it is filtered out
	assert.Nil(t, gotNetB)

	gotGNPB, err := gnpLister.Get("gnp-b")
	assert.Error(t, err)
	assert.Nil(t, gotGNPB)
}

func TestFilteredNetworkSharedInformerFactory_SupervisorAccess(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fakeNetworking := networkfake.NewSimpleClientset()
	parentFactory := networkinformers.NewSharedInformerFactory(fakeNetworking, 0*time.Second)

	labelKey := "tenancy.gke.io/provider-config"
	// supervisor has allowMissing = true, filterValue = "supervisor"
	filteredFactory := NewFilteredNetworkSharedInformerFactory(parentFactory, labelKey, "supervisor", true)

	// Resources:
	// 1. Labeled supervisor
	netSup := &apinetworkv1.Network{
		ObjectMeta: metav1.ObjectMeta{
			Name: "net-sup",
			Labels: map[string]string{
				labelKey: "supervisor",
			},
		},
	}
	// 2. Labeled tenant-A
	netTenant := &apinetworkv1.Network{
		ObjectMeta: metav1.ObjectMeta{
			Name: "net-tenant-a",
			Labels: map[string]string{
				labelKey: "tenant-A",
			},
		},
	}
	// 3. No label (missing label)
	netMissing := &apinetworkv1.Network{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "net-missing-label",
			Labels: map[string]string{},
		},
	}

	_, err := fakeNetworking.NetworkingV1().Networks().Create(ctx, netSup, metav1.CreateOptions{})
	assert.NoError(t, err)
	_, err = fakeNetworking.NetworkingV1().Networks().Create(ctx, netTenant, metav1.CreateOptions{})
	assert.NoError(t, err)
	_, err = fakeNetworking.NetworkingV1().Networks().Create(ctx, netMissing, metav1.CreateOptions{})
	assert.NoError(t, err)

	netInformer := filteredFactory.Networking().V1().Networks()
	_ = netInformer.Informer()

	stopCh := make(chan struct{})
	defer close(stopCh)
	parentFactory.Start(stopCh)
	parentFactory.WaitForCacheSync(stopCh)

	netLister := netInformer.Lister()

	nets, err := netLister.List(labels.Everything())
	assert.NoError(t, err)

	// Should contain net-sup and net-missing-label (since allowMissing: true)
	// net-tenant-a should be filtered out
	assert.Len(t, nets, 2)
	names := []string{nets[0].Name, nets[1].Name}
	assert.Contains(t, names, "net-sup")
	assert.Contains(t, names, "net-missing-label")
	assert.NotContains(t, names, "net-tenant-a")
}

func TestFilteredNetworkSharedInformerFactory_CacheImmutability(t *testing.T) {
	fakeNetworking := networkfake.NewSimpleClientset()
	parentFactory := networkinformers.NewSharedInformerFactory(fakeNetworking, 0*time.Second)
	filteredFactory := NewFilteredNetworkSharedInformerFactory(parentFactory, "key", "val", false)

	inf := filteredFactory.Networking().V1().Networks().Informer()
	cacheStore := inf.GetStore()

	assert.ErrorContains(t, cacheStore.Add(&apinetworkv1.Network{}), "read-only filtered cache")
	assert.ErrorContains(t, cacheStore.Update(&apinetworkv1.Network{}), "read-only filtered cache")
	assert.ErrorContains(t, cacheStore.Delete(&apinetworkv1.Network{}), "read-only filtered cache")
	assert.ErrorContains(t, cacheStore.Replace(nil, ""), "read-only filtered cache")
	assert.ErrorContains(t, cacheStore.Resync(), "read-only filtered cache")
}

func TestFilteredNetworkSharedInformerFactory_Indexing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fakeNetworking := networkfake.NewSimpleClientset()
	parentFactory := networkinformers.NewSharedInformerFactory(fakeNetworking, 0*time.Second)
	parentFactory.Networking().V1().Networks().Informer().AddIndexers(cache.Indexers{
		"test-index": func(obj interface{}) ([]string, error) {
			net := obj.(*apinetworkv1.Network)
			return []string{string(net.Spec.Type)}, nil
		},
	})

	filteredFactory := NewFilteredNetworkSharedInformerFactory(parentFactory, "tenant", "A", false)

	netA := &apinetworkv1.Network{
		ObjectMeta: metav1.ObjectMeta{Name: "net-a", Labels: map[string]string{"tenant": "A"}},
		Spec:       apinetworkv1.NetworkSpec{Type: "L3"},
	}
	fakeNetworking.NetworkingV1().Networks().Create(ctx, netA, metav1.CreateOptions{})
	netB := &apinetworkv1.Network{
		ObjectMeta: metav1.ObjectMeta{Name: "net-b", Labels: map[string]string{"tenant": "B"}},
		Spec:       apinetworkv1.NetworkSpec{Type: "L3"},
	}
	fakeNetworking.NetworkingV1().Networks().Create(ctx, netB, metav1.CreateOptions{})

	stopCh := make(chan struct{})
	defer close(stopCh)
	parentFactory.Start(stopCh)
	parentFactory.WaitForCacheSync(stopCh)

	inf := filteredFactory.Networking().V1().Networks().Informer()
	indexer := inf.GetIndexer()

	keys, err := indexer.IndexKeys("test-index", "L3")
	assert.NoError(t, err)
	assert.Len(t, keys, 1)
	assert.Contains(t, keys, "net-a")

	vals := indexer.ListIndexFuncValues("test-index")
	assert.Contains(t, vals, "L3")
}

func TestFilteredNetworkSharedInformerFactory_DeletedFinalStateUnknown(t *testing.T) {
	fakeNetworking := networkfake.NewSimpleClientset()
	parentFactory := networkinformers.NewSharedInformerFactory(fakeNetworking, 0*time.Second)
	filteredFactory := NewFilteredNetworkSharedInformerFactory(parentFactory, "tenant", "A", false)

	inf := filteredFactory.Networking().V1().Networks().Informer()
	// Using reflection or type assertion if possible, but since it's an interface, we can test FilterFunc directly
	// Actually we can't access localFilteredInformer since it's private, we have to check its behavior.
	// However, they are in the same package `utils`.
	localInf, ok := inf.(*localFilteredInformer)
	assert.True(t, ok)

	validObj := &apinetworkv1.Network{
		ObjectMeta: metav1.ObjectMeta{Name: "net-a", Labels: map[string]string{"tenant": "A"}},
	}
	invalidObj := &apinetworkv1.Network{
		ObjectMeta: metav1.ObjectMeta{Name: "net-b", Labels: map[string]string{"tenant": "B"}},
	}

	assert.True(t, localInf.FilterFunc(validObj))
	assert.False(t, localInf.FilterFunc(invalidObj))

	unknownValid := cache.DeletedFinalStateUnknown{Key: "net-a", Obj: validObj}
	unknownInvalid := cache.DeletedFinalStateUnknown{Key: "net-b", Obj: invalidObj}

	assert.True(t, localInf.FilterFunc(unknownValid))
	assert.False(t, localInf.FilterFunc(unknownInvalid))
}

func TestFilteredNetworkSharedInformerFactory_SingletonAndConcurrency(t *testing.T) {
	fakeNetworking := networkfake.NewSimpleClientset()
	parentFactory := networkinformers.NewSharedInformerFactory(fakeNetworking, 0*time.Second)
	filteredFactory := NewFilteredNetworkSharedInformerFactory(parentFactory, "tenant", "A", false)

	netInformer := filteredFactory.Networking().V1().Networks()

	inf1 := netInformer.Informer()
	inf2 := netInformer.Informer()
	assert.Same(t, inf1, inf2)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			inf1.AddEventHandler(cache.ResourceEventHandlerFuncs{})
		}()
	}
	wg.Wait()

	filteredFactory.Cleanup()
}
