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

package dynamicpodip

import (
	"context"
	"errors"
	"testing"
	"time"

	nncv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/nodenetworkconfig/v1"
	nncfake "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/clientset/versioned/fake"
	nncinformers "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/informers/externalversions"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/clock"
)

func countANEs(t *testing.T, backend PodIPBackend) int {
	t.Helper()
	ifaces, err := backend.GetNetworkInterfaces(context.Background(), testProviderID)
	if err != nil {
		t.Fatalf("GetNetworkInterfaces: %v", err)
	}
	n := 0
	for _, iface := range ifaces {
		n += len(iface.ANECIDRs)
	}
	return n
}

// scriptedDemand returns an AdditionDemandFunc that hands out the given
// increments in order, then reports no further demand.
func scriptedDemand(increments ...int) (AdditionDemandFunc, *int) {
	calls := 0
	i := 0
	return func(context.Context) ([]string, error) {
		calls++
		if i >= len(increments) {
			return nil, nil
		}
		n := increments[i]
		i++
		adds := make([]string, n)
		for j := range adds {
			adds[j] = "/32"
		}
		return adds, nil
	}, &calls
}

func TestANEBackendMutateWithTopUp_AbsorbsDemand(t *testing.T) {
	backend := NewANEBackend(newFakeGCEWithDefaultInstance(t)).(TopUpBackend)

	more, calls := scriptedDemand(5, 2)
	err := backend.MutateWithTopUp(context.Background(), testProviderID, testNetworkURL, []string{"/32", "/32", "/32"}, nil, nil, more)
	if err != nil {
		t.Fatalf("MutateWithTopUp: %v", err)
	}
	if got := countANEs(t, backend); got != 10 {
		t.Fatalf("expected 3 initial + 5 + 2 topped-up = 10 ANEs, got %d", got)
	}
	if *calls < 3 {
		t.Fatalf("expected the demand function to be consulted until it reported no demand (>=3 calls), got %d", *calls)
	}
}

func TestANEBackendMutateWithTopUp_NilDemandMatchesMutate(t *testing.T) {
	backend := NewANEBackend(newFakeGCEWithDefaultInstance(t)).(TopUpBackend)
	if err := backend.MutateWithTopUp(context.Background(), testProviderID, testNetworkURL, []string{"/32", "/32"}, nil, nil, nil); err != nil {
		t.Fatalf("MutateWithTopUp(nil demand): %v", err)
	}
	if got := countANEs(t, backend); got != 2 {
		t.Fatalf("expected 2 ANEs, got %d", got)
	}
}

func TestANEBackendMutateWithTopUp_DemandErrorIsNotFatal(t *testing.T) {
	backend := NewANEBackend(newFakeGCEWithDefaultInstance(t)).(TopUpBackend)
	more := func(context.Context) ([]string, error) { return nil, errors.New("lister unavailable") }
	if err := backend.MutateWithTopUp(context.Background(), testProviderID, testNetworkURL, []string{"/32"}, nil, nil, more); err != nil {
		t.Fatalf("a failing demand function must not fail the mutation, got: %v", err)
	}
	if got := countANEs(t, backend); got != 1 {
		t.Fatalf("expected the initial ANE to be created regardless, got %d", got)
	}
}

func TestANEBackendMutateWithTopUp_CapsTopUp(t *testing.T) {
	backend := NewANEBackend(newFakeGCEWithDefaultInstance(t)).(TopUpBackend)
	// A demand function that never stops asking.
	more := func(context.Context) ([]string, error) { return []string{"/32", "/32", "/32"}, nil }
	if err := backend.MutateWithTopUp(context.Background(), testProviderID, testNetworkURL, []string{"/32"}, nil, nil, more); err != nil {
		t.Fatalf("MutateWithTopUp: %v", err)
	}
	if got := countANEs(t, backend); got != 1+maxANETopUpPerMutate {
		t.Fatalf("expected top-up to be capped at %d (+1 initial), got %d ANEs", maxANETopUpPerMutate, got)
	}
}

func TestANEBackendMutateWithTopUp_NoTopUpNearDeadline(t *testing.T) {
	backend := NewANEBackend(newFakeGCEWithDefaultInstance(t)).(TopUpBackend)
	// Less than DefaultANEOperationTimeout left: the initial batch still
	// runs, but no new work may be started.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	more, calls := scriptedDemand(4)
	if err := backend.MutateWithTopUp(ctx, testProviderID, testNetworkURL, []string{"/32", "/32"}, nil, nil, more); err != nil {
		t.Fatalf("MutateWithTopUp: %v", err)
	}
	if got := countANEs(t, backend); got != 2 {
		t.Fatalf("expected only the 2 initial ANEs near the deadline, got %d", got)
	}
	if *calls != 0 {
		t.Fatalf("expected the demand function not to be consulted near the deadline, got %d calls", *calls)
	}
}

// bumpOnMutateBackend simulates the daemon raising spec.allocations while
// the CCM is mid-mutation: before delegating, it rewrites the NNC in the
// informer store with a higher generation and ask.
type bumpOnMutateBackend struct {
	TopUpBackend
	store   func(*nncv1.NodeNetworkConfig)
	bumped  *nncv1.NodeNetworkConfig
	mutates int
}

func (b *bumpOnMutateBackend) MutateWithTopUp(ctx context.Context, providerID, networkURL string, additions, removals, candidateRanges []string, more AdditionDemandFunc) error {
	b.mutates++
	if b.bumped != nil {
		b.store(b.bumped)
		b.bumped = nil
	}
	return b.TopUpBackend.MutateWithTopUp(ctx, providerID, networkURL, additions, removals, candidateRanges, more)
}

func TestSpecControllerTopsUpOnSpecBumpDuringMutation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	kubeClient := k8sfake.NewSimpleClientset()
	nncClient := nncfake.NewSimpleClientset()
	nodeInformer := informers.NewSharedInformerFactory(kubeClient, 0).Core().V1().Nodes()
	nncInformer := nncinformers.NewSharedInformerFactory(nncClient, 0).Networking().V1().NodeNetworkConfigs()

	fakeGCE := newFakeGCEWithDefaultInstance(t)
	inner := NewANEBackend(fakeGCE).(TopUpBackend)
	bumping := &bumpOnMutateBackend{
		TopUpBackend: inner,
		store: func(n *nncv1.NodeNetworkConfig) {
			if err := nncInformer.Informer().GetStore().Update(n); err != nil {
				t.Fatalf("update NNC in cache: %v", err)
			}
		},
	}
	gceCache := NewGCECache(func(ctx context.Context, providerID string) ([]*networkInterface, error) {
		return inner.GetNetworkInterfaces(ctx, providerID)
	}, 10*time.Second, clock.RealClock{})
	specCtrl := NewSpecController(kubeClient, nncClient, nncInformer, nodeInformer, fakeGCE, gceCache, nil, nil, bumping)

	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: testNodeName}, Spec: corev1.NodeSpec{ProviderID: testProviderID}}
	if _, err := kubeClient.CoreV1().Nodes().Create(ctx, node, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Create Node: %v", err)
	}
	if err := nodeInformer.Informer().GetStore().Add(node); err != nil {
		t.Fatalf("Add Node to cache: %v", err)
	}

	// Snapshot the reconcile will see: ask for 2.
	nnc := &nncv1.NodeNetworkConfig{
		ObjectMeta: metav1.ObjectMeta{Name: testNodeName, Generation: 1},
		Spec:       nncv1.NodeNetworkConfigSpec{Allocations: []nncv1.Allocation{{Network: "default", Pods: 2}}},
	}
	if _, err := nncClient.NetworkingV1().NodeNetworkConfigs().Create(ctx, nnc, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Create NNC: %v", err)
	}
	if err := nncInformer.Informer().GetStore().Add(nnc); err != nil {
		t.Fatalf("Add NNC to cache: %v", err)
	}
	// What the daemon will have written by the time the mutation is in
	// flight: ask for 7.
	bumped := nnc.DeepCopy()
	bumped.Generation = 2
	bumped.Spec.Allocations[0].Pods = 7
	bumping.bumped = bumped

	if err := specCtrl.syncNode(testNodeName); err != nil {
		t.Fatalf("syncNode: %v", err)
	}

	if bumping.mutates != 1 {
		t.Fatalf("expected exactly one mutation, got %d", bumping.mutates)
	}
	if got := countANEs(t, inner); got != 7 {
		t.Fatalf("expected the bump to 7 to be absorbed into the single mutation (7 ANEs), got %d", got)
	}

	// A follow-up reconcile for the bumped generation must find nothing to do.
	gceCache.Invalidate(testNodeName)
	if err := specCtrl.syncNode(testNodeName); err != nil {
		t.Fatalf("second syncNode: %v", err)
	}
	if bumping.mutates != 1 {
		t.Fatalf("expected no further mutation on the follow-up reconcile, got %d total", bumping.mutates)
	}
	if got := countANEs(t, inner); got != 7 {
		t.Fatalf("expected 7 ANEs after follow-up reconcile, got %d", got)
	}
}

func TestAdditionDemandFunc_BaselineAndIncrements(t *testing.T) {
	kubeClient := k8sfake.NewSimpleClientset()
	nncClient := nncfake.NewSimpleClientset()
	nodeInformer := informers.NewSharedInformerFactory(kubeClient, 0).Core().V1().Nodes()
	nncInformer := nncinformers.NewSharedInformerFactory(nncClient, 0).Networking().V1().NodeNetworkConfigs()
	fakeGCE := newFakeGCEWithDefaultInstance(t)
	backend := NewANEBackend(fakeGCE)
	c := NewSpecController(kubeClient, nncClient, nncInformer, nodeInformer, fakeGCE, nil, nil, nil, backend)

	// Capacity 16 (one /28), snapshot ask 20 -> baseline 20.
	ifaces := []*networkInterface{{Name: "nic0", Network: testNetworkURL, AliasIPRanges: []string{"10.0.0.0/28"}}}
	snapshot := &nncv1.NodeNetworkConfig{
		ObjectMeta: metav1.ObjectMeta{Name: testNodeName, Generation: 5},
		Spec:       nncv1.NodeNetworkConfigSpec{Allocations: []nncv1.Allocation{{Network: "default", Pods: 20}}},
	}
	if err := nncInformer.Informer().GetStore().Add(snapshot); err != nil {
		t.Fatal(err)
	}
	more := c.additionDemandFunc(snapshot, "default", ifaces)

	set := func(gen int64, pods int32) {
		n := snapshot.DeepCopy()
		n.Generation = gen
		n.Spec.Allocations[0].Pods = pods
		if err := nncInformer.Informer().GetStore().Update(n); err != nil {
			t.Fatal(err)
		}
	}
	want := func(step string, n int) {
		t.Helper()
		adds, err := more(context.Background())
		if err != nil {
			t.Fatalf("%s: %v", step, err)
		}
		if len(adds) != n {
			t.Fatalf("%s: expected %d additions, got %v", step, n, adds)
		}
	}

	want("same generation", 0)
	set(6, 20)
	want("new generation, same ask", 0)
	set(7, 26)
	want("ask 20->26", 6)
	want("repeat without change", 0)
	set(8, 24)
	want("ask lowered below what was handed out", 0)
	set(9, 30)
	want("ask 24->30 only counts beyond 26 already handed out", 4)

	// Snapshot ask below capacity: baseline is capacity.
	low := snapshot.DeepCopy()
	low.Generation = 20
	low.Spec.Allocations[0].Pods = 10
	moreLow := c.additionDemandFunc(low, "default", ifaces)
	set(21, 18)
	adds, err := moreLow(context.Background())
	if err != nil || len(adds) != 2 {
		t.Fatalf("expected 18-16 = 2 additions when snapshot ask was below capacity, got %v, %v", adds, err)
	}
}
