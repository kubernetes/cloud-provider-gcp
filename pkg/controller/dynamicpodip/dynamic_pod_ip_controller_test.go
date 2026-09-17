/*
Copyright 2026 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package dynamicpodip

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	networkv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/network/v1"
	nncv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/nodenetworkconfig/v1"
	nncfake "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/clientset/versioned/fake"
	nncinformers "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/informers/externalversions"
	gcloud "github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud"
	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/meta"
	computebeta "google.golang.org/api/compute/v0.beta"
	compute "google.golang.org/api/compute/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/informers"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	clientgotesting "k8s.io/client-go/testing"
	gce "k8s.io/cloud-provider-gcp/providers/gce"
	clocktesting "k8s.io/utils/clock/testing"
)

const (
	testNodeName = "test-node"
	testZone     = "us-central1-a"
	testProject  = "test-project"
)

var (
	testProviderID = fmt.Sprintf("gce://%s/%s/%s", testProject, testZone, testNodeName)
	testNetworkURL = fmt.Sprintf("https://www.googleapis.com/compute/v1/projects/%s/global/networks/%s", testProject, "default")
)

type testFixture struct {
	t                   *testing.T
	kubeClient          *k8sfake.Clientset
	nncClient           *nncfake.Clientset
	informerFactory     nncinformers.SharedInformerFactory
	nodeInformerFactory informers.SharedInformerFactory
	fakeGCE             *gce.Cloud
	specCtrl            *NodeNetworkConfigSpecController
	statusCtrl          *NodeNetworkConfigStatusController
	fakeClock           *clocktesting.FakeClock
	gceCalls            *ifaceUpdateRecorder
}

func newTestFixture(t *testing.T) *testFixture {
	kubeClient := k8sfake.NewSimpleClientset()
	nncClient := nncfake.NewSimpleClientset()

	// The fake tracker replaces the entire stored object on a subresource
	// write, unlike a real apiserver, which discards spec changes sent to
	// /status. Left alone, a status write built from a stale copy silently
	// resurrects spec fields another writer just removed -- precisely the
	// interleaving the releasable-CIDR prune exists to survive, so the fake
	// would hide the bug it is meant to catch. Preserve the stored spec.
	//
	// Note this must not call back into nncClient: Fake.Invokes holds the
	// clientset lock across the reactor, so a nested client call deadlocks.
	// Go through the tracker, which has its own lock and is not held here.
	nncClient.PrependReactor("update", "nodenetworkconfigs", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		if action.GetSubresource() != "status" {
			return false, nil, nil
		}
		updateAction, ok := action.(clientgotesting.UpdateAction)
		if !ok {
			return false, nil, nil
		}
		incoming, ok := updateAction.GetObject().(*nncv1.NodeNetworkConfig)
		if !ok {
			return false, nil, nil
		}
		gvr := action.GetResource()
		existingObj, err := nncClient.Tracker().Get(gvr, action.GetNamespace(), incoming.Name)
		if err != nil {
			// Let the default reactor surface the error.
			return false, nil, nil
		}
		existing, ok := existingObj.(*nncv1.NodeNetworkConfig)
		if !ok {
			return false, nil, nil
		}
		merged := incoming.DeepCopy()
		merged.Spec = *existing.Spec.DeepCopy()
		if err := nncClient.Tracker().Update(gvr, merged, action.GetNamespace()); err != nil {
			return true, nil, err
		}
		return true, merged, nil
	})

	informerFactory := nncinformers.NewSharedInformerFactory(nncClient, 0)
	nncInformer := informerFactory.Networking().V1().NodeNetworkConfigs()

	// Create fake Node informer
	fakeInformerFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	nodeInformer := fakeInformerFactory.Core().V1().Nodes()

	testClusterValues := gce.DefaultTestClusterValues()
	testClusterValues.ProjectID = testProject
	testClusterValues.ZoneName = testZone
	testClusterValues.NetworkURL = testNetworkURL
	fakeGCE := gce.NewFakeGCECloud(testClusterValues)

	// Register the UpdateNetworkInterface hook to simulate GCE mutation and allocation
	mockInstances, ok := fakeGCE.Compute().BetaInstances().(*gcloud.MockBetaInstances)
	if !ok {
		t.Fatalf("Failed to cast BetaInstances to MockBetaInstances")
	}
	// Record every interface update before applying it, so tests can assert on
	// how a reconcile was split into requests rather than only on the state it
	// left behind.
	recorder := &ifaceUpdateRecorder{}
	mockInstances.UpdateNetworkInterfaceHook = func(
		ctx context.Context,
		key *meta.Key,
		ifaceName string,
		iface *computebeta.NetworkInterface,
		mock *gcloud.MockBetaInstances,
		options ...gcloud.Option,
	) error {
		recorder.record(ifaceName, iface)
		return updateNetworkInterfaceHook(ctx, key, ifaceName, iface, mock, options...)
	}

	loader := func(ctx context.Context, providerID string) ([]*networkInterface, error) {
		gceIfaces, err := fakeGCE.GetInstanceNetworkInterfaces(ctx, providerID)
		if err != nil {
			return nil, err
		}
		return toNetworkInterfaces(gceIfaces), nil
	}

	initialTime := time.Date(2026, 6, 26, 9, 0, 0, 0, time.UTC)
	fakeClock := clocktesting.NewFakeClock(initialTime)

	gceCache := NewGCECache(loader, 10*time.Second, fakeClock)

	statusCtrl := NewStatusController(
		kubeClient,
		nncClient,
		nncInformer.Lister(),
		nodeInformer,
		fakeGCE,
		gceCache,
		fakeClock,
	)

	specCtrl := NewSpecController(
		kubeClient,
		nncClient,
		nncInformer,
		nodeInformer,
		fakeGCE,
		gceCache,
		statusCtrl,
	)

	return &testFixture{
		t:                   t,
		kubeClient:          kubeClient,
		nncClient:           nncClient,
		informerFactory:     informerFactory,
		nodeInformerFactory: fakeInformerFactory,
		fakeGCE:             fakeGCE,
		specCtrl:            specCtrl,
		statusCtrl:          statusCtrl,
		fakeClock:           fakeClock,
		gceCalls:            recorder,
	}
}

func (f *testFixture) run(ctx context.Context, stopCh <-chan struct{}) {
	f.informerFactory.Start(stopCh)
}

// runNodeInformer starts and syncs the Node informer. Kept separate from run so
// that tests which do not exercise Node events are not perturbed by the status
// controller's Node handler firing.
func (f *testFixture) runNodeInformer(stopCh <-chan struct{}) {
	f.t.Helper()
	f.nodeInformerFactory.Start(stopCh)
	f.nodeInformerFactory.WaitForCacheSync(stopCh)
}

func (f *testFixture) reconcile(ctx context.Context, nnc *nncv1.NodeNetworkConfig, providerID string) error {
	err := f.specCtrl.reconcile(ctx, nnc, providerID)
	if err != nil {
		return err
	}
	return f.statusCtrl.reconcile(ctx, nnc, providerID)
}

func TestReconcile_AddAliasIP(t *testing.T) {
	ctx := context.Background()
	f := newTestFixture(t)
	stopCh := make(chan struct{})
	defer close(stopCh)
	f.run(ctx, stopCh)

	// Create a fake GCE instance in the fake cloud
	instanceKey := meta.ZonalKey(testNodeName, testZone)
	instance := &compute.Instance{
		Name: testNodeName,
		Zone: testZone,
		NetworkInterfaces: []*compute.NetworkInterface{
			{
				Name:       "nic0",
				Network:    testNetworkURL,
				Subnetwork: "default",
			},
		},
	}
	// Insert GA instance (fake GCE shares DB between GA and Beta)
	err := f.fakeGCE.Compute().Instances().Insert(ctx, instanceKey, instance)
	if err != nil {
		t.Fatalf("Failed to insert fake GCE instance: %v", err)
	}

	// Create a NodeNetworkConfig with a request for 48 pods
	nnc := &nncv1.NodeNetworkConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNodeName,
		},
		Spec: nncv1.NodeNetworkConfigSpec{
			Allocations: []nncv1.Allocation{
				{
					Network: "default",
					Pods:    48, // Requests 48 pods -> should result in 3x /28 blocks (48 IPs)
				},
			},
		},
	}
	_, err = f.nncClient.NetworkingV1().NodeNetworkConfigs().Create(ctx, nnc, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create NodeNetworkConfig: %v", err)
	}

	// Sync informer cache
	f.informerFactory.WaitForCacheSync(stopCh)

	// Run reconcile
	err = f.reconcile(ctx, nnc, testProviderID)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	// Verify GCE Instance has new alias IPs
	updatedInstance, err := f.fakeGCE.Compute().BetaInstances().Get(ctx, instanceKey)
	if err != nil {
		t.Fatalf("Failed to get updated GCE instance: %v", err)
	}

	iface := updatedInstance.NetworkInterfaces[0]
	// We expect 3 alias IP ranges added (each of size /28)
	if len(iface.AliasIpRanges) != 3 {
		t.Errorf("Expected 3 alias IP ranges, got %d: %v", len(iface.AliasIpRanges), iface.AliasIpRanges)
	}
	expectedCIDRs := []string{"10.100.0.0/28", "10.100.1.0/28", "10.100.2.0/28"}
	for i, r := range iface.AliasIpRanges {
		if i < len(expectedCIDRs) && r.IpCidrRange != expectedCIDRs[i] {
			t.Errorf("Expected alias IP range %q, got %q", expectedCIDRs[i], r.IpCidrRange)
		}
	}

	// Verify NodeNetworkConfig Status is updated
	updatedNNC, err := f.nncClient.NetworkingV1().NodeNetworkConfigs().Get(ctx, testNodeName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get updated NodeNetworkConfig: %v", err)
	}

	if len(updatedNNC.Status.PodCIDRs) != 3 {
		t.Errorf("Expected 3 PodCIDRs in status, got %d: %v", len(updatedNNC.Status.PodCIDRs), updatedNNC.Status.PodCIDRs)
	}
	for i, pc := range updatedNNC.Status.PodCIDRs {
		if i < len(expectedCIDRs) && pc.CIDR != expectedCIDRs[i] {
			t.Errorf("Expected PodCIDR %q, got %q", expectedCIDRs[i], pc.CIDR)
		}
		if pc.Network != "default" {
			t.Errorf("Expected network 'default', got %q", pc.Network)
		}
		if pc.Condition == nil || pc.Condition.Status != metav1.ConditionTrue {
			t.Errorf("Expected PodCIDR condition to be True, got: %v", pc.Condition)
		}
	}

	// Verify overall condition is Ready
	readyCond := getCondition(updatedNNC.Status.Conditions, string(nncv1.NodeNetworkConfigConditionReady))
	if readyCond == nil || readyCond.Status != metav1.ConditionTrue {
		t.Errorf("Expected NodeNetworkConfig Ready condition to be True, got: %v", readyCond)
	}
}

func TestReconcile_RemoveAliasIP(t *testing.T) {
	ctx := context.Background()
	f := newTestFixture(t)
	stopCh := make(chan struct{})
	defer close(stopCh)
	f.run(ctx, stopCh)

	cidrToRemove := "10.100.0.0/28"
	cidrToKeep := "10.100.1.0/28"

	// Create a fake GCE instance with 2 alias IPs
	instanceKey := meta.ZonalKey(testNodeName, testZone)
	instance := &compute.Instance{
		Name: testNodeName,
		Zone: testZone,
		NetworkInterfaces: []*compute.NetworkInterface{
			{
				Name:       "nic0",
				Network:    testNetworkURL,
				Subnetwork: "default",
				AliasIpRanges: []*compute.AliasIpRange{
					{IpCidrRange: cidrToRemove},
					{IpCidrRange: cidrToKeep},
				},
			},
		},
	}
	err := f.fakeGCE.Compute().Instances().Insert(ctx, instanceKey, instance)
	if err != nil {
		t.Fatalf("Failed to insert fake GCE instance: %v", err)
	}

	// Create NodeNetworkConfig with:
	// - Status containing both CIDRs
	// - Spec.ReleasableCIDRs containing cidrToRemove
	nnc := &nncv1.NodeNetworkConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNodeName,
		},
		Spec: nncv1.NodeNetworkConfigSpec{
			ReleasableCIDRs: []nncv1.PodCIDR{
				{
					Network: "default",
					CIDR:    cidrToRemove,
				},
			},
		},
		Status: nncv1.NodeNetworkConfigStatus{
			PodCIDRs: []nncv1.PodCIDR{
				{Id: cidrToRemove, Network: "default", CIDR: cidrToRemove},
				{Id: cidrToKeep, Network: "default", CIDR: cidrToKeep},
			},
			Conditions: []metav1.Condition{
				{
					Type:               string(nncv1.NodeNetworkConfigConditionReady),
					Status:             metav1.ConditionTrue,
					LastTransitionTime: metav1.Now(),
					Reason:             string(nncv1.NodeNetworkConfigReadyReason),
				},
			},
		},
	}
	_, err = f.nncClient.NetworkingV1().NodeNetworkConfigs().Create(ctx, nnc, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create NodeNetworkConfig: %v", err)
	}

	// Sync informer cache
	f.informerFactory.WaitForCacheSync(stopCh)

	// Run reconcile
	err = f.reconcile(ctx, nnc, testProviderID)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	// Verify GCE Instance has only the kept alias IP
	updatedInstance, err := f.fakeGCE.Compute().BetaInstances().Get(ctx, instanceKey)
	if err != nil {
		t.Fatalf("Failed to get updated GCE instance: %v", err)
	}

	iface := updatedInstance.NetworkInterfaces[0]
	if len(iface.AliasIpRanges) != 1 {
		t.Errorf("Expected 1 alias IP range, got %d: %v", len(iface.AliasIpRanges), iface.AliasIpRanges)
	}
	if iface.AliasIpRanges[0].IpCidrRange != cidrToKeep {
		t.Errorf("Expected kept range %q, got %q", cidrToKeep, iface.AliasIpRanges[0].IpCidrRange)
	}

	// Verify NodeNetworkConfig Status is updated (only kept CIDR remains)
	updatedNNC, err := f.nncClient.NetworkingV1().NodeNetworkConfigs().Get(ctx, testNodeName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get updated NodeNetworkConfig: %v", err)
	}

	if len(updatedNNC.Status.PodCIDRs) != 1 {
		t.Errorf("Expected 1 PodCIDR in status, got %d: %v", len(updatedNNC.Status.PodCIDRs), updatedNNC.Status.PodCIDRs)
	}
	if updatedNNC.Status.PodCIDRs[0].CIDR != cidrToKeep {
		t.Errorf("Expected kept PodCIDR %q, got %q", cidrToKeep, updatedNNC.Status.PodCIDRs[0].CIDR)
	}
}

func TestReconcile_NoOp(t *testing.T) {
	ctx := context.Background()
	f := newTestFixture(t)
	stopCh := make(chan struct{})
	defer close(stopCh)
	f.run(ctx, stopCh)

	cidr := "10.100.0.0/28" // 16 IPs

	// Create GCE instance with 1 alias IP
	instanceKey := meta.ZonalKey(testNodeName, testZone)
	instance := &compute.Instance{
		Name: testNodeName,
		Zone: testZone,
		NetworkInterfaces: []*compute.NetworkInterface{
			{
				Name:       "nic0",
				Network:    testNetworkURL,
				Subnetwork: "default",
				AliasIpRanges: []*compute.AliasIpRange{
					{IpCidrRange: cidr},
				},
			},
		},
	}
	err := f.fakeGCE.Compute().Instances().Insert(ctx, instanceKey, instance)
	if err != nil {
		t.Fatalf("Failed to insert fake GCE instance: %v", err)
	}

	// Create NodeNetworkConfig where Spec matches Status capacity (16 desired, 16 actual)
	nnc := &nncv1.NodeNetworkConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNodeName,
		},
		Spec: nncv1.NodeNetworkConfigSpec{
			Allocations: []nncv1.Allocation{
				{
					Network: "default",
					Pods:    16, // Matches current capacity of 16
				},
			},
		},
		Status: nncv1.NodeNetworkConfigStatus{
			PodCIDRs: []nncv1.PodCIDR{
				{Id: cidr, Network: "default", CIDR: cidr},
			},
			Conditions: []metav1.Condition{
				{
					Type:               string(nncv1.NodeNetworkConfigConditionReady),
					Status:             metav1.ConditionTrue,
					LastTransitionTime: metav1.Now(),
					Reason:             string(nncv1.NodeNetworkConfigReadyReason),
				},
			},
		},
	}
	_, err = f.nncClient.NetworkingV1().NodeNetworkConfigs().Create(ctx, nnc, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create NodeNetworkConfig: %v", err)
	}

	// Sync informer cache
	f.informerFactory.WaitForCacheSync(stopCh)

	// Track GCE calls by checking if fingerprint changes (it shouldn't because no updates should be made)
	// But simpler: fake GCE doesn't track call counts easily unless we mock.
	// We can just verify that the instance in fake GCE remains unchanged.

	// Run reconcile
	err = f.reconcile(ctx, nnc, testProviderID)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	// Verify GCE Instance remains unchanged
	updatedInstance, err := f.fakeGCE.Compute().BetaInstances().Get(ctx, instanceKey)
	if err != nil {
		t.Fatalf("Failed to get GCE instance: %v", err)
	}
	if len(updatedInstance.NetworkInterfaces[0].AliasIpRanges) != 1 {
		t.Errorf("Expected 1 alias IP range, got %d", len(updatedInstance.NetworkInterfaces[0].AliasIpRanges))
	}
}

func TestReconcile_GCEError(t *testing.T) {
	ctx := context.Background()
	f := newTestFixture(t)
	stopCh := make(chan struct{})
	defer close(stopCh)
	f.run(ctx, stopCh)

	// We do NOT create the GCE instance. This will cause the GCE Get call to fail
	// with InstanceNotFound, simulating a GCE API error (or rather, a configuration/sync error).

	// Create NodeNetworkConfig with a request
	nnc := &nncv1.NodeNetworkConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNodeName,
		},
		Spec: nncv1.NodeNetworkConfigSpec{
			Allocations: []nncv1.Allocation{
				{
					Network: "default",
					Pods:    32,
				},
			},
		},
	}
	_, err := f.nncClient.NetworkingV1().NodeNetworkConfigs().Create(ctx, nnc, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create NodeNetworkConfig: %v", err)
	}

	// Sync informer cache
	f.informerFactory.WaitForCacheSync(stopCh)

	// Run reconcile. It should fail because the instance doesn't exist in GCE.
	err = f.reconcile(ctx, nnc, testProviderID)
	if err == nil {
		t.Fatal("Expected reconcile to fail, but it succeeded")
	}

	// Verify NodeNetworkConfig Status has False Ready condition with correct reason
	updatedNNC, err := f.nncClient.NetworkingV1().NodeNetworkConfigs().Get(ctx, testNodeName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get updated NodeNetworkConfig: %v", err)
	}

	readyCond := getCondition(updatedNNC.Status.Conditions, string(nncv1.NodeNetworkConfigConditionReady))
	if readyCond == nil {
		t.Fatal("Expected Ready condition to be present")
	}
	if readyCond.Status != metav1.ConditionFalse {
		t.Errorf("Expected Ready condition to be False, got %s", readyCond.Status)
	}
	expectedReason := string(nncv1.NodeNetworkConfigInvalidParametersReason)
	if readyCond.Reason != expectedReason {
		t.Errorf("Expected reason %q, got %q", expectedReason, readyCond.Reason)
	}
}

func getCondition(conditions []metav1.Condition, cType string) *metav1.Condition {
	for _, c := range conditions {
		if c.Type == cType {
			return &c
		}
	}
	return nil
}

// resolveMockAliasIPs simulates GCE IP allocation for range sizes (like "/28").
func resolveMockAliasIPs(ranges []*computebeta.AliasIpRange) []*computebeta.AliasIpRange {
	result := []*computebeta.AliasIpRange{}
	existingMap := make(map[string]bool)

	// First pass: collect all valid existing CIDRs to avoid conflicts
	for _, r := range ranges {
		if r.IpCidrRange != "" && r.IpCidrRange[0] != '/' {
			existingMap[r.IpCidrRange] = true
		}
	}

	// Second pass: resolve the ones starting with '/'
	nextSubnet := 0
	for _, r := range ranges {
		if r.IpCidrRange != "" && r.IpCidrRange[0] == '/' {
			size := r.IpCidrRange // e.g. "/28"
			var candidate string
			for {
				candidate = fmt.Sprintf("10.100.%d.0%s", nextSubnet, size)
				nextSubnet++
				if !existingMap[candidate] {
					break
				}
			}
			result = append(result, &computebeta.AliasIpRange{
				IpCidrRange:         candidate,
				SubnetworkRangeName: r.SubnetworkRangeName,
			})
			existingMap[candidate] = true
		} else {
			result = append(result, r)
		}
	}
	return result
}

// mockFingerprintSeq hands out interface fingerprints for the mock. It is
// monotonic rather than derived from the interface's contents so that two
// updates leaving the same number of ranges still produce distinct tokens.
var mockFingerprintSeq atomic.Int64

// ifaceUpdateCall is one updateNetworkInterface request as the provider issued
// it, before the mock resolved any range sizes into concrete CIDRs.
type ifaceUpdateCall struct {
	ifaceName string
	ranges    []string
}

// ifaceUpdateRecorder captures the sequence of updateNetworkInterface requests.
// The final state of an instance cannot distinguish one combined request from
// an add and a remove issued separately, and GCE rejects the combined form, so
// that distinction has to be asserted on the requests themselves.
type ifaceUpdateRecorder struct {
	mu    sync.Mutex
	calls []ifaceUpdateCall
}

func (r *ifaceUpdateRecorder) record(ifaceName string, iface *computebeta.NetworkInterface) {
	ranges := make([]string, 0, len(iface.AliasIpRanges))
	for _, a := range iface.AliasIpRanges {
		ranges = append(ranges, a.IpCidrRange)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, ifaceUpdateCall{ifaceName: ifaceName, ranges: ranges})
}

// snapshot returns a copy of the calls recorded so far.
func (r *ifaceUpdateRecorder) snapshot() []ifaceUpdateCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]ifaceUpdateCall{}, r.calls...)
}

// updateNetworkInterfaceHook implements the GCE mutation in the mock store.
func updateNetworkInterfaceHook(
	ctx context.Context,
	key *meta.Key,
	ifaceName string,
	iface *computebeta.NetworkInterface,
	mock *gcloud.MockBetaInstances,
	options ...gcloud.Option,
) error {
	mock.Lock.Lock()
	defer mock.Lock.Unlock()

	obj, ok := mock.Objects[*key]
	if !ok {
		return fmt.Errorf("instance %v not found in mock store", key)
	}

	instance := obj.ToBeta()

	// Find the target interface and update it
	updated := false
	for i, ni := range instance.NetworkInterfaces {
		if ni.Name == ifaceName {
			// GCE guards updateNetworkInterface with the interface
			// fingerprint and rotates it on every successful update, so a
			// caller that issues two updates in a row has to re-read the
			// instance in between. Enforce that here: without it, reusing a
			// stale fingerprint is invisible to these tests. Instances seeded
			// by tests carry no fingerprint, so the first update is exempt.
			if ni.Fingerprint != "" && iface.Fingerprint != ni.Fingerprint {
				return fmt.Errorf("stale fingerprint for interface %q on instance %v: got %q, want %q", ifaceName, key, iface.Fingerprint, ni.Fingerprint)
			}
			// Resolve the alias IPs (allocate for '/28' sizes)
			resolvedRanges := resolveMockAliasIPs(iface.AliasIpRanges)
			instance.NetworkInterfaces[i].AliasIpRanges = resolvedRanges
			instance.NetworkInterfaces[i].Fingerprint = fmt.Sprintf("fingerprint-%d", mockFingerprintSeq.Add(1))
			updated = true
			break
		}
	}

	if !updated {
		return fmt.Errorf("network interface %q not found on mock instance %v", ifaceName, key)
	}

	// Write back to mock store
	mock.Objects[*key] = &gcloud.MockInstancesObj{Obj: instance}
	return nil
}

func TestReconcile_InvalidStatusCIDR(t *testing.T) {
	ctx := context.Background()
	f := newTestFixture(t)
	stopCh := make(chan struct{})
	defer close(stopCh)
	f.run(ctx, stopCh)

	// Create a fake GCE instance with 0 alias IPs (clean state)
	instanceKey := meta.ZonalKey(testNodeName, testZone)
	instance := &compute.Instance{
		Name: testNodeName,
		Zone: testZone,
		NetworkInterfaces: []*compute.NetworkInterface{
			{
				Name:       "nic0",
				Network:    testNetworkURL,
				Subnetwork: "default",
			},
		},
	}
	err := f.fakeGCE.Compute().Instances().Insert(ctx, instanceKey, instance)
	if err != nil {
		t.Fatalf("Failed to insert fake GCE instance: %v", err)
	}

	// Part 1: Create NNC with an invalid CIDR in Status.
	// The controller should NOT fail; it should heal the status to match GCE (allocating 2 blocks).
	nncInvalid := &nncv1.NodeNetworkConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNodeName,
		},
		Spec: nncv1.NodeNetworkConfigSpec{
			Allocations: []nncv1.Allocation{
				{Network: "default", Pods: 32},
			},
		},
		Status: nncv1.NodeNetworkConfigStatus{
			PodCIDRs: []nncv1.PodCIDR{
				{Id: "invalid-cidr", Network: "default", CIDR: "invalid-cidr"},
			},
		},
	}
	_, err = f.nncClient.NetworkingV1().NodeNetworkConfigs().Create(ctx, nncInvalid, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create NNC: %v", err)
	}

	f.informerFactory.WaitForCacheSync(stopCh)

	// Run reconcile. It should succeed because GCE is valid, and it should heal K8s status.
	err = f.reconcile(ctx, nncInvalid, testProviderID)
	if err != nil {
		t.Fatalf("Expected reconcile to succeed (self-healing), got error: %v", err)
	}

	// Verify GCE has exactly 2 blocks allocated
	updatedInstance, err := f.fakeGCE.Compute().BetaInstances().Get(ctx, instanceKey)
	if err != nil {
		t.Fatalf("Failed to get GCE instance: %v", err)
	}
	if len(updatedInstance.NetworkInterfaces[0].AliasIpRanges) != 2 {
		t.Fatalf("Expected GCE to have 2 alias IPs allocated, got %d", len(updatedInstance.NetworkInterfaces[0].AliasIpRanges))
	}

	// Verify NNC Status is healed: has exactly 2 valid blocks, and "invalid-cidr" is GONE
	updatedNNC, err := f.nncClient.NetworkingV1().NodeNetworkConfigs().Get(ctx, testNodeName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get NNC: %v", err)
	}
	if len(updatedNNC.Status.PodCIDRs) != 2 {
		t.Fatalf("Expected Status to be healed to 2 PodCIDRs, got %d: %v", len(updatedNNC.Status.PodCIDRs), updatedNNC.Status.PodCIDRs)
	}
	for _, pc := range updatedNNC.Status.PodCIDRs {
		if pc.CIDR == "invalid-cidr" {
			t.Errorf("FAIL: 'invalid-cidr' was not cleaned up from status")
		}
	}
	readyCond := getCondition(updatedNNC.Status.Conditions, string(nncv1.NodeNetworkConfigConditionReady))
	if readyCond == nil || readyCond.Status != metav1.ConditionTrue {
		t.Errorf("Expected Ready condition to be True, got: %v", readyCond)
	}

	// Part 2: IPv6 CIDR in Status.
	// Clean up NNC and GCE alias IPs to reset.
	mockInstances, _ := f.fakeGCE.Compute().BetaInstances().(*gcloud.MockBetaInstances)
	instObj, err := mockInstances.Get(ctx, instanceKey)
	if err != nil {
		t.Fatalf("Failed to get instance from mock: %v", err)
	}
	mockInstances.Lock.Lock()
	instObj.NetworkInterfaces[0].AliasIpRanges = nil // reset GCE
	mockInstances.Lock.Unlock()

	err = f.nncClient.NetworkingV1().NodeNetworkConfigs().Delete(ctx, testNodeName, metav1.DeleteOptions{})
	if err != nil {
		t.Fatalf("Failed to delete NNC: %v", err)
	}

	// Create NNC with IPv6 CIDR in Status
	nncIPv6 := &nncv1.NodeNetworkConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNodeName,
		},
		Spec: nncv1.NodeNetworkConfigSpec{
			Allocations: []nncv1.Allocation{
				{Network: "default", Pods: 32},
			},
		},
		Status: nncv1.NodeNetworkConfigStatus{
			PodCIDRs: []nncv1.PodCIDR{
				{Id: "2001:db8::/64", Network: "default", CIDR: "2001:db8::/64"},
			},
		},
	}
	_, err = f.nncClient.NetworkingV1().NodeNetworkConfigs().Create(ctx, nncIPv6, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create NNC: %v", err)
	}
	f.informerFactory.WaitForCacheSync(stopCh)

	// Invalidate cache manually (since we reset GCE behind its back, and it might still be fresh from the previous test!)
	// Wait, we don't have Invalidate anymore!
	// How do we expire the cache?
	// We can just advance the fake clock by 11 seconds!
	f.fakeClock.Step(11 * time.Second)

	// Run reconcile. It should succeed and heal status (allocating 2 blocks).
	err = f.reconcile(ctx, nncIPv6, testProviderID)
	if err != nil {
		t.Fatalf("Expected reconcile to succeed (self-healing IPv6), got error: %v", err)
	}

	// Verify GCE has 2 blocks
	updatedInstance2, _ := f.fakeGCE.Compute().BetaInstances().Get(ctx, instanceKey)
	if len(updatedInstance2.NetworkInterfaces[0].AliasIpRanges) != 2 {
		t.Fatalf("Expected GCE to have 2 alias IPs allocated (IPv6 test), got %d", len(updatedInstance2.NetworkInterfaces[0].AliasIpRanges))
	}

	// Verify NNC Status is healed: has exactly 2 valid blocks, and IPv6 is GONE
	updatedNNC2, _ := f.nncClient.NetworkingV1().NodeNetworkConfigs().Get(ctx, testNodeName, metav1.GetOptions{})
	if len(updatedNNC2.Status.PodCIDRs) != 2 {
		t.Fatalf("Expected Status to be healed to 2 PodCIDRs (IPv6 test), got %d: %v", len(updatedNNC2.Status.PodCIDRs), updatedNNC2.Status.PodCIDRs)
	}
	for _, pc := range updatedNNC2.Status.PodCIDRs {
		if pc.CIDR == "2001:db8::/64" {
			t.Errorf("FAIL: IPv6 CIDR was not cleaned up from status")
		}
	}
}

func TestReconcile_IdempotentRetryOnStatusFailure(t *testing.T) {
	ctx := context.Background()
	f := newTestFixture(t)
	stopCh := make(chan struct{})
	defer close(stopCh)
	f.run(ctx, stopCh)

	// Create a fake GCE instance in the fake cloud
	instanceKey := meta.ZonalKey(testNodeName, testZone)
	instance := &compute.Instance{
		Name: testNodeName,
		Zone: testZone,
		NetworkInterfaces: []*compute.NetworkInterface{
			{
				Name:       "nic0",
				Network:    testNetworkURL,
				Subnetwork: "default",
			},
		},
	}
	err := f.fakeGCE.Compute().Instances().Insert(ctx, instanceKey, instance)
	if err != nil {
		t.Fatalf("Failed to insert fake GCE instance: %v", err)
	}

	// Create a NodeNetworkConfig with a request for 32 pods (needs 2 blocks of /28)
	nnc := &nncv1.NodeNetworkConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNodeName,
		},
		Spec: nncv1.NodeNetworkConfigSpec{
			Allocations: []nncv1.Allocation{
				{
					Network: "default",
					Pods:    32,
				},
			},
		},
	}
	_, err = f.nncClient.NetworkingV1().NodeNetworkConfigs().Create(ctx, nnc, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create NodeNetworkConfig: %v", err)
	}

	f.informerFactory.WaitForCacheSync(stopCh)

	// Inject a reactor to fail the second Status Update (the final "Ready" status update, after GCE call)
	statusUpdateCount := 0
	f.nncClient.PrependReactor("update", "nodenetworkconfigs", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		if action.GetSubresource() == "status" {
			statusUpdateCount++
			if statusUpdateCount == 2 {
				return true, nil, fmt.Errorf("simulated status conflict error")
			}
		}
		return false, nil, nil
	})

	// First reconcile run. It should fail on the status update.
	err = f.reconcile(ctx, nnc, testProviderID)
	if err == nil {
		t.Fatal("Expected first reconcile to fail due to simulated status update error, but it succeeded")
	}

	// GCE call succeeded, so GCE should have 2 blocks allocated now.
	updatedInstance, err := f.fakeGCE.Compute().BetaInstances().Get(ctx, instanceKey)
	if err != nil {
		t.Fatalf("Failed to get GCE instance: %v", err)
	}
	if len(updatedInstance.NetworkInterfaces[0].AliasIpRanges) != 2 {
		t.Fatalf("Expected 2 alias IP ranges in GCE after first run, got %d", len(updatedInstance.NetworkInterfaces[0].AliasIpRanges))
	}
	allocatedCIDRs := []string{
		updatedInstance.NetworkInterfaces[0].AliasIpRanges[0].IpCidrRange,
		updatedInstance.NetworkInterfaces[0].AliasIpRanges[1].IpCidrRange,
	}

	// But NNC Status was NOT updated because of the reactor error.
	unsyncedNNC, err := f.nncClient.NetworkingV1().NodeNetworkConfigs().Get(ctx, testNodeName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get NNC: %v", err)
	}
	if len(unsyncedNNC.Status.PodCIDRs) != 0 {
		t.Fatalf("Expected 0 PodCIDRs in status due to update failure, got %d", len(unsyncedNNC.Status.PodCIDRs))
	}

	// Second reconcile run (Simulated Retry).
	// The reactor will now succeed.
	// We pass the same NNC object (which still has empty status).
	err = f.reconcile(ctx, unsyncedNNC, testProviderID)
	if err != nil {
		t.Fatalf("Second reconcile (retry) failed: %v", err)
	}

	// ASSERTION: GCE should STILL only have 2 blocks allocated!
	// If the controller is not idempotent, it will have allocated more blocks (e.g. 4)!
	finalInstance, err := f.fakeGCE.Compute().BetaInstances().Get(ctx, instanceKey)
	if err != nil {
		t.Fatalf("Failed to get GCE instance: %v", err)
	}
	if len(finalInstance.NetworkInterfaces[0].AliasIpRanges) != 2 {
		t.Errorf("FAIL: Expected GCE to still have exactly 2 alias IP ranges, but it has %d: %v",
			len(finalInstance.NetworkInterfaces[0].AliasIpRanges), finalInstance.NetworkInterfaces[0].AliasIpRanges)
	}

	// Status should have exactly those 2 blocks.
	finalNNC, err := f.nncClient.NetworkingV1().NodeNetworkConfigs().Get(ctx, testNodeName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get NNC: %v", err)
	}
	if len(finalNNC.Status.PodCIDRs) != 2 {
		t.Errorf("FAIL: Expected 2 PodCIDRs in status, got %d: %v", len(finalNNC.Status.PodCIDRs), finalNNC.Status.PodCIDRs)
	} else {
		for i, pc := range finalNNC.Status.PodCIDRs {
			if i < len(allocatedCIDRs) && pc.CIDR != allocatedCIDRs[i] {
				t.Errorf("FAIL: Expected PodCIDR to match %q, got %q", allocatedCIDRs[i], pc.CIDR)
			}
		}
	}
}

func TestReconcile_MultiNetwork(t *testing.T) {
	ctx := context.Background()
	f := newTestFixture(t)
	stopCh := make(chan struct{})
	defer close(stopCh)
	f.run(ctx, stopCh)

	customNetworkName := "custom-network"
	customNetworkURL := fmt.Sprintf("https://www.googleapis.com/compute/v1/projects/%s/global/networks/%s", testProject, customNetworkName)

	// Create a GCE instance with TWO network interfaces
	instanceKey := meta.ZonalKey(testNodeName, testZone)
	instance := &compute.Instance{
		Name: testNodeName,
		Zone: testZone,
		NetworkInterfaces: []*compute.NetworkInterface{
			{
				Name:       "nic0",
				Network:    testNetworkURL,
				Subnetwork: "default",
			},
			{
				Name:       "nic1",
				Network:    customNetworkURL,
				Subnetwork: "custom-subnet",
			},
		},
	}
	err := f.fakeGCE.Compute().Instances().Insert(ctx, instanceKey, instance)
	if err != nil {
		t.Fatalf("Failed to insert fake GCE instance: %v", err)
	}

	// Create NodeNetworkConfig requesting allocations on BOTH networks
	nnc := &nncv1.NodeNetworkConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNodeName,
		},
		Spec: nncv1.NodeNetworkConfigSpec{
			Allocations: []nncv1.Allocation{
				{
					Network: "default",
					Pods:    32, // needs 2 blocks of /28
				},
				{
					Network: customNetworkName,
					Pods:    32, // needs 2 blocks of /28
				},
			},
		},
	}
	_, err = f.nncClient.NetworkingV1().NodeNetworkConfigs().Create(ctx, nnc, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create NodeNetworkConfig: %v", err)
	}

	f.informerFactory.WaitForCacheSync(stopCh)

	// Run reconcile
	err = f.reconcile(ctx, nnc, testProviderID)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	// Verify GCE Instance has alias IPs on BOTH interfaces
	updatedInstance, err := f.fakeGCE.Compute().BetaInstances().Get(ctx, instanceKey)
	if err != nil {
		t.Fatalf("Failed to get updated GCE instance: %v", err)
	}

	if len(updatedInstance.NetworkInterfaces) != 2 {
		t.Fatalf("Expected 2 network interfaces, got %d", len(updatedInstance.NetworkInterfaces))
	}

	// nic0 (default) - expects 2 blocks of /28 for 32 pods
	nic0 := updatedInstance.NetworkInterfaces[0]
	if len(nic0.AliasIpRanges) != 2 {
		t.Errorf("Expected 2 alias IP ranges on nic0, got %d: %v", len(nic0.AliasIpRanges), nic0.AliasIpRanges)
	}

	// nic1 (custom) - expects 2 blocks of /28 for 32 pods
	nic1 := updatedInstance.NetworkInterfaces[1]
	if len(nic1.AliasIpRanges) != 2 {
		t.Errorf("Expected 2 alias IP ranges on nic1, got %d: %v", len(nic1.AliasIpRanges), nic1.AliasIpRanges)
	}

	// Verify both allocations are in NNC Status
	updatedNNC, err := f.nncClient.NetworkingV1().NodeNetworkConfigs().Get(ctx, testNodeName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get updated NodeNetworkConfig: %v", err)
	}

	if len(updatedNNC.Status.PodCIDRs) != 4 {
		t.Errorf("Expected 4 PodCIDRs in status, got %d: %v", len(updatedNNC.Status.PodCIDRs), updatedNNC.Status.PodCIDRs)
	}

	// Verify they are mapped to the correct networks
	defaultCount := 0
	customCount := 0
	for _, pc := range updatedNNC.Status.PodCIDRs {
		if pc.Network == "default" {
			defaultCount++
		} else if pc.Network == customNetworkName {
			customCount++
		} else {
			t.Errorf("Unexpected network %q in status PodCIDR", pc.Network)
		}
	}
	if defaultCount != 2 || customCount != 2 {
		t.Errorf("Expected 2 default and 2 custom PodCIDRs, got default=%d, custom=%d", defaultCount, customCount)
	}
}

func TestReconcile_CacheExpirationBehavior(t *testing.T) {
	ctx := context.Background()
	f := newTestFixture(t)
	stopCh := make(chan struct{})
	defer close(stopCh)
	f.run(ctx, stopCh)

	// Create a fake GCE instance
	instanceKey := meta.ZonalKey(testNodeName, testZone)
	instance := &compute.Instance{
		Name: testNodeName,
		Zone: testZone,
		NetworkInterfaces: []*compute.NetworkInterface{
			{
				Name:       "nic0",
				Network:    testNetworkURL,
				Subnetwork: "default",
			},
		},
	}
	err := f.fakeGCE.Compute().Instances().Insert(ctx, instanceKey, instance)
	if err != nil {
		t.Fatalf("Failed to insert fake GCE instance: %v", err)
	}

	// Create NNC requesting 32 pods (needs 2 blocks of /28)
	nnc := &nncv1.NodeNetworkConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNodeName,
		},
		Spec: nncv1.NodeNetworkConfigSpec{
			Allocations: []nncv1.Allocation{
				{
					Network: "default",
					Pods:    32,
				},
			},
		},
	}
	_, err = f.nncClient.NetworkingV1().NodeNetworkConfigs().Create(ctx, nnc, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create NodeNetworkConfig: %v", err)
	}

	f.informerFactory.WaitForCacheSync(stopCh)

	// Inject reactor to fail the second status update (final "Ready" write)
	statusUpdateCount := 0
	f.nncClient.PrependReactor("update", "nodenetworkconfigs", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		if action.GetSubresource() == "status" {
			statusUpdateCount++
			if statusUpdateCount == 2 {
				return true, nil, fmt.Errorf("simulated status conflict error")
			}
		}
		return false, nil, nil
	})

	// First run: allocates 2 blocks in GCE, fails status write.
	err = f.reconcile(ctx, nnc, testProviderID)
	if err == nil {
		t.Fatal("Expected reconcile to fail on status write, but it succeeded")
	}

	// Verify GCE has 2 blocks
	updatedInstance, err := f.fakeGCE.Compute().BetaInstances().Get(ctx, instanceKey)
	if err != nil {
		t.Fatalf("Failed to get GCE instance: %v", err)
	}
	if len(updatedInstance.NetworkInterfaces[0].AliasIpRanges) != 2 {
		t.Fatalf("Expected 2 alias IP ranges in GCE, got %d", len(updatedInstance.NetworkInterfaces[0].AliasIpRanges))
	}

	// Manually mutate GCE behind the back of the cache (add a 3rd block!)
	// This simulates out-of-band changes or GCE state drift.
	mockInstances, _ := f.fakeGCE.Compute().BetaInstances().(*gcloud.MockBetaInstances)
	instObj, err := mockInstances.Get(ctx, instanceKey)
	if err != nil {
		t.Fatalf("Failed to get instance from mock: %v", err)
	}
	mockInstances.Lock.Lock()
	instObj.NetworkInterfaces[0].AliasIpRanges = append(instObj.NetworkInterfaces[0].AliasIpRanges, &computebeta.AliasIpRange{
		IpCidrRange:         "10.100.2.0/28",
		SubnetworkRangeName: "default-secondary",
	})
	mockInstances.Lock.Unlock()

	// Scenario A: Retry immediately (Fresh Cache, age = 0s < 10s TTL)
	// The simulated reactor only fails on the 2nd update, so this retry will naturally succeed.

	unsyncedNNC, err := f.nncClient.NetworkingV1().NodeNetworkConfigs().Get(ctx, testNodeName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get NNC: %v", err)
	}

	err = f.reconcile(ctx, unsyncedNNC, testProviderID)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	// ASSERTION: Status controller uses ForceGet to fetch fresh GCE state, so it immediately discovers the 3rd block!
	finalNNC, err := f.nncClient.NetworkingV1().NodeNetworkConfigs().Get(ctx, testNodeName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get NNC: %v", err)
	}
	if len(finalNNC.Status.PodCIDRs) != 3 {
		t.Errorf("Expected 3 PodCIDRs in status (ForceGet), got %d: %v", len(finalNNC.Status.PodCIDRs), finalNNC.Status.PodCIDRs)
	}

	// Scenario B: Expire the Cache & Sync again
	// Update Spec to 48 pods (needs 3 blocks)
	finalNNC.Spec.Allocations[0].Pods = 48
	_, err = f.nncClient.NetworkingV1().NodeNetworkConfigs().Update(ctx, finalNNC, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("Failed to update NNC Spec: %v", err)
	}
	f.informerFactory.WaitForCacheSync(stopCh)

	// Advance the clock by 11 seconds (cache is now STALE!)
	f.fakeClock.Step(11 * time.Second)

	// Run reconciliation
	updatedNNC2, _ := f.nncClient.NetworkingV1().NodeNetworkConfigs().Get(ctx, testNodeName, metav1.GetOptions{})
	err = f.reconcile(ctx, updatedNNC2, testProviderID)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	// ASSERTION: GCE should STILL only have 3 blocks!
	// Because the cache expired, the controller did a GCE GET, saw the 3rd block was already there,
	// calculated a diff of 0, and skipped the GCE mutation!
	// If it had hit a stale cache that only knew about 2 blocks, it would have called GCE to add a 4th block!
	finalInstance2, err := f.fakeGCE.Compute().BetaInstances().Get(ctx, instanceKey)
	if err != nil {
		t.Fatalf("Failed to get GCE instance: %v", err)
	}
	if len(finalInstance2.NetworkInterfaces[0].AliasIpRanges) != 3 {
		t.Errorf("FAIL: Cache expiration failed to refresh from GCE. Expected 3 alias IP ranges, got %d: %v",
			len(finalInstance2.NetworkInterfaces[0].AliasIpRanges), finalInstance2.NetworkInterfaces[0].AliasIpRanges)
	}

	// Status should have all 3 blocks
	finalNNC2, err := f.nncClient.NetworkingV1().NodeNetworkConfigs().Get(ctx, testNodeName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get NNC: %v", err)
	}
	if len(finalNNC2.Status.PodCIDRs) != 3 {
		t.Errorf("Expected 3 PodCIDRs in status, got %d: %v", len(finalNNC2.Status.PodCIDRs), finalNNC2.Status.PodCIDRs)
	}
}

func TestStatusController_Independent(t *testing.T) {
	ctx := context.Background()
	f := newTestFixture(t)

	// Create a GCE instance with existing alias IPs
	instanceKey := meta.ZonalKey(testNodeName, testZone)
	instance := &compute.Instance{
		Name: testNodeName,
		Zone: testZone,
		NetworkInterfaces: []*compute.NetworkInterface{
			{
				Name:       "nic0",
				Network:    testNetworkURL,
				Subnetwork: "default",
				AliasIpRanges: []*compute.AliasIpRange{
					{IpCidrRange: "10.100.0.0/28"},
				},
			},
		},
	}
	err := f.fakeGCE.Compute().Instances().Insert(ctx, instanceKey, instance)
	if err != nil {
		t.Fatalf("Failed to insert fake GCE instance: %v", err)
	}

	// Create NNC with empty Status
	nnc := &nncv1.NodeNetworkConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNodeName,
		},
	}
	_, err = f.nncClient.NetworkingV1().NodeNetworkConfigs().Create(ctx, nnc, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create NodeNetworkConfig: %v", err)
	}

	// Create a node object in fake kubeClient
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNodeName,
		},
		Spec: corev1.NodeSpec{
			ProviderID: testProviderID,
		},
	}
	_, err = f.kubeClient.CoreV1().Nodes().Create(ctx, node, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Failed to create Node: %v", err)
	}

	// Directly invoke status controller syncNode (simulating workqueue execution)
	err = f.statusCtrl.syncNode(testNodeName)
	if err != nil {
		t.Fatalf("Status sync failed: %v", err)
	}

	// Verify NNC Status is populated
	updatedNNC, err := f.nncClient.NetworkingV1().NodeNetworkConfigs().Get(ctx, testNodeName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get updated NNC: %v", err)
	}

	if len(updatedNNC.Status.PodCIDRs) != 1 {
		t.Fatalf("Expected 1 PodCIDR in status, got %d", len(updatedNNC.Status.PodCIDRs))
	}
	if updatedNNC.Status.PodCIDRs[0].CIDR != "10.100.0.0/28" {
		t.Errorf("Expected PodCIDR 10.100.0.0/28, got %q", updatedNNC.Status.PodCIDRs[0].CIDR)
	}
}

func TestNoopStatusTrigger(t *testing.T) {
	trigger := &NoopStatusTrigger{}
	// Calling EnqueueNode on NoopStatusTrigger should execute cleanly without panic
	trigger.EnqueueNode("test-node")
}

func TestSpecController_UpdateFilterSkipsUnchangedGeneration(t *testing.T) {
	f := newTestFixture(t)

	base := &nncv1.NodeNetworkConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:            testNodeName,
			Generation:      3,
			ResourceVersion: "1000",
		},
		Spec: nncv1.NodeNetworkConfigSpec{
			Allocations: []nncv1.Allocation{{Network: "default", Pods: 16}},
		},
	}

	// A status write bumps ResourceVersion but not Generation. Both controllers write status,
	// so without this filter each real change would re-enqueue the spec controller twice.
	statusWrite := base.DeepCopy()
	statusWrite.ResourceVersion = "1001"
	statusWrite.Status.PodCIDRs = []nncv1.PodCIDR{
		{Id: "10.100.0.0/28", Network: "default", CIDR: "10.100.0.0/28"},
	}
	f.specCtrl.handleNodeNetworkConfigUpdate(base, statusWrite)
	if got := f.specCtrl.queue.Len(); got != 0 {
		t.Errorf("Expected no enqueue for a status-only update, got queue length %d", got)
	}

	// A watch relist redelivers the identical object as an update.
	f.specCtrl.handleNodeNetworkConfigUpdate(base, base.DeepCopy())
	if got := f.specCtrl.queue.Len(); got != 0 {
		t.Errorf("Expected no enqueue for a relist of an unchanged object, got queue length %d", got)
	}

	// A real spec change bumps Generation and must still be processed.
	specChange := base.DeepCopy()
	specChange.Generation = 4
	specChange.ResourceVersion = "1002"
	specChange.Spec.Allocations = []nncv1.Allocation{{Network: "default", Pods: 32}}
	f.specCtrl.handleNodeNetworkConfigUpdate(base, specChange)
	if got := f.specCtrl.queue.Len(); got != 1 {
		t.Errorf("Expected 1 enqueue for a spec change, got queue length %d", got)
	}
}

func TestSpecController_UpdateFilterFailsOpenOnUnexpectedType(t *testing.T) {
	f := newTestFixture(t)

	nnc := &nncv1.NodeNetworkConfig{
		ObjectMeta: metav1.ObjectMeta{Name: testNodeName, Generation: 1},
	}

	// If the old object is not a NodeNetworkConfig we cannot compare generations, so the update
	// must be enqueued rather than silently dropped.
	f.specCtrl.handleNodeNetworkConfigUpdate("not-a-nodenetworkconfig", nnc)
	if got := f.specCtrl.queue.Len(); got != 1 {
		t.Errorf("Expected the update to be enqueued when the type is unexpected, got queue length %d", got)
	}
}

// TestSpecController_RequeuesWhenNodeNotReady covers the window where a NodeNetworkConfig is
// reconciled before its Node has registered. The spec controller only watches NodeNetworkConfigs,
// so if the key were forgotten here nothing would ever reconcile this node again.
func TestSpecController_RequeuesWhenNodeNotReady(t *testing.T) {
	ctx := context.Background()
	f := newTestFixture(t)

	nnc := &nncv1.NodeNetworkConfig{
		ObjectMeta: metav1.ObjectMeta{Name: testNodeName},
		Spec: nncv1.NodeNetworkConfigSpec{
			Allocations: []nncv1.Allocation{{Network: "default", Pods: 16}},
		},
	}
	if _, err := f.nncClient.NetworkingV1().NodeNetworkConfigs().Create(ctx, nnc, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create NodeNetworkConfig: %v", err)
	}

	// Case 1: the Node object does not exist yet.
	err := f.specCtrl.syncNode(testNodeName)
	if _, ok := err.(*nodeNotReadyError); !ok {
		t.Fatalf("Expected *nodeNotReadyError when the node is missing, got %T: %v", err, err)
	}

	// Case 2: the Node exists but has not been assigned a ProviderID yet.
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: testNodeName}}
	if _, err := f.kubeClient.CoreV1().Nodes().Create(ctx, node, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Node: %v", err)
	}
	err = f.specCtrl.syncNode(testNodeName)
	if _, ok := err.(*nodeNotReadyError); !ok {
		t.Fatalf("Expected *nodeNotReadyError when ProviderID is empty, got %T: %v", err, err)
	}

	// The work item must be requeued rather than forgotten.
	f.specCtrl.queue.Add(testNodeName)
	f.specCtrl.processNextWorkItem()
	if got := f.specCtrl.queue.NumRequeues(testNodeName); got != 1 {
		t.Errorf("Expected the key to be requeued once, got %d requeues", got)
	}
}

func TestStatusController_RequeuesWhenNodeNotReady(t *testing.T) {
	ctx := context.Background()
	f := newTestFixture(t)

	nnc := &nncv1.NodeNetworkConfig{ObjectMeta: metav1.ObjectMeta{Name: testNodeName}}
	if _, err := f.nncClient.NetworkingV1().NodeNetworkConfigs().Create(ctx, nnc, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create NodeNetworkConfig: %v", err)
	}

	err := f.statusCtrl.syncNode(testNodeName)
	if _, ok := err.(*nodeNotReadyError); !ok {
		t.Fatalf("Expected *nodeNotReadyError when the node is missing, got %T: %v", err, err)
	}

	f.statusCtrl.queue.Add(testNodeName)
	f.statusCtrl.processNextWorkItem()
	if got := f.statusCtrl.queue.NumRequeues(testNodeName); got != 1 {
		t.Errorf("Expected the key to be requeued once, got %d requeues", got)
	}
}

// insertInstanceWithRanges creates a fake GCE instance carrying the given alias
// IP ranges on nic0.
func insertInstanceWithRanges(ctx context.Context, t *testing.T, f *testFixture, cidrs ...string) *meta.Key {
	t.Helper()

	var ranges []*compute.AliasIpRange
	for _, cidr := range cidrs {
		ranges = append(ranges, &compute.AliasIpRange{IpCidrRange: cidr})
	}
	instanceKey := meta.ZonalKey(testNodeName, testZone)
	instance := &compute.Instance{
		Name: testNodeName,
		Zone: testZone,
		NetworkInterfaces: []*compute.NetworkInterface{
			{
				Name:          "nic0",
				Network:       testNetworkURL,
				Subnetwork:    "default",
				AliasIpRanges: ranges,
			},
		},
	}
	if err := f.fakeGCE.Compute().Instances().Insert(ctx, instanceKey, instance); err != nil {
		t.Fatalf("Failed to insert fake GCE instance: %v", err)
	}
	return instanceKey
}

// releasableCIDRsOf returns the CIDRs currently listed in Spec.ReleasableCIDRs.
func releasableCIDRsOf(ctx context.Context, t *testing.T, f *testFixture) []string {
	t.Helper()

	nnc, err := f.nncClient.NetworkingV1().NodeNetworkConfigs().Get(ctx, testNodeName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get NodeNetworkConfig: %v", err)
	}
	var got []string
	for _, rel := range nnc.Spec.ReleasableCIDRs {
		got = append(got, rel.CIDR)
	}
	sort.Strings(got)
	return got
}

// countGCEWrites installs a reactor counting UpdateNetworkInterface calls, so a
// test can assert that a reconcile pass touched GCE (or didn't).
func countGCEWrites(t *testing.T, f *testFixture) *int {
	t.Helper()

	mockInstances, ok := f.fakeGCE.Compute().BetaInstances().(*gcloud.MockBetaInstances)
	if !ok {
		t.Fatalf("Failed to cast BetaInstances to MockBetaInstances")
	}
	calls := 0
	inner := mockInstances.UpdateNetworkInterfaceHook
	mockInstances.UpdateNetworkInterfaceHook = func(ctx context.Context, key *meta.Key, ifaceName string, iface *computebeta.NetworkInterface, m *gcloud.MockBetaInstances, options ...gcloud.Option) error {
		calls++
		return inner(ctx, key, ifaceName, iface, m, options...)
	}
	return &calls
}

// TestPruneReleasableCIDRs_AfterSuccessfulRemoval covers the happy path: the
// range is deleted from GCE and its entry is dropped from the spec in the same
// pass.
func TestPruneReleasableCIDRs_AfterSuccessfulRemoval(t *testing.T) {
	ctx := context.Background()
	f := newTestFixture(t)
	stopCh := make(chan struct{})
	defer close(stopCh)
	f.run(ctx, stopCh)

	const (
		cidrToRemove = "10.100.1.0/28"
		cidrToKeep   = "10.100.0.0/28"
	)
	instanceKey := insertInstanceWithRanges(ctx, t, f, cidrToRemove, cidrToKeep)

	nnc := &nncv1.NodeNetworkConfig{
		ObjectMeta: metav1.ObjectMeta{Name: testNodeName},
		Spec: nncv1.NodeNetworkConfigSpec{
			Allocations: []nncv1.Allocation{{Network: "default", Pods: 16}},
			ReleasableCIDRs: []nncv1.PodCIDR{
				{Id: cidrToRemove, Network: "default", CIDR: cidrToRemove},
			},
		},
	}
	if _, err := f.nncClient.NetworkingV1().NodeNetworkConfigs().Create(ctx, nnc, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create NodeNetworkConfig: %v", err)
	}
	f.informerFactory.WaitForCacheSync(stopCh)

	if err := f.reconcile(ctx, nnc, testProviderID); err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	updated, err := f.fakeGCE.Compute().BetaInstances().Get(ctx, instanceKey)
	if err != nil {
		t.Fatalf("Failed to get GCE instance: %v", err)
	}
	if len(updated.NetworkInterfaces[0].AliasIpRanges) != 1 {
		t.Fatalf("Expected 1 alias IP range in GCE, got %d", len(updated.NetworkInterfaces[0].AliasIpRanges))
	}

	if got := releasableCIDRsOf(ctx, t, f); len(got) != 0 {
		t.Errorf("Expected Spec.ReleasableCIDRs to be pruned, got %v", got)
	}
}

// TestPruneReleasableCIDRs_ToleratesDaemonReAdd covers the accepted race where
// the daemon re-emits an entry because it has not yet seen the CIDR leave
// status. The next pass must prune it again without calling GCE.
func TestPruneReleasableCIDRs_ToleratesDaemonReAdd(t *testing.T) {
	ctx := context.Background()
	f := newTestFixture(t)
	stopCh := make(chan struct{})
	defer close(stopCh)
	f.run(ctx, stopCh)

	const released = "10.100.1.0/28"
	insertInstanceWithRanges(ctx, t, f, "10.100.0.0/28")

	// The range is already gone from GCE, but the daemon has (re-)added an entry
	// for it because its own view still shows it in status.
	nnc := &nncv1.NodeNetworkConfig{
		ObjectMeta: metav1.ObjectMeta{Name: testNodeName},
		Spec: nncv1.NodeNetworkConfigSpec{
			Allocations: []nncv1.Allocation{{Network: "default", Pods: 16}},
			ReleasableCIDRs: []nncv1.PodCIDR{
				{Id: released, Network: "default", CIDR: released},
			},
		},
	}
	if _, err := f.nncClient.NetworkingV1().NodeNetworkConfigs().Create(ctx, nnc, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create NodeNetworkConfig: %v", err)
	}
	f.informerFactory.WaitForCacheSync(stopCh)

	gceWrites := countGCEWrites(t, f)

	if err := f.reconcile(ctx, nnc, testProviderID); err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	if got := releasableCIDRsOf(ctx, t, f); len(got) != 0 {
		t.Errorf("Expected the re-added entry to be pruned, got %v", got)
	}
	// The whole point: an entry naming a range GCE no longer has must not
	// provoke a mutation.
	if *gceWrites != 0 {
		t.Errorf("Expected no GCE writes for an already-released range, got %d", *gceWrites)
	}
}

// TestPruneReleasableCIDRs_RecoversAfterCrash simulates dying between the GCE
// deletion and the spec write. A later pass must prune from GCE state alone.
func TestPruneReleasableCIDRs_RecoversAfterCrash(t *testing.T) {
	ctx := context.Background()
	f := newTestFixture(t)
	stopCh := make(chan struct{})
	defer close(stopCh)
	f.run(ctx, stopCh)

	const released = "10.100.1.0/28"
	insertInstanceWithRanges(ctx, t, f, "10.100.0.0/28")

	// Status still lists the released range, exactly as it would after a crash
	// before either the spec prune or the status resync happened.
	nnc := &nncv1.NodeNetworkConfig{
		ObjectMeta: metav1.ObjectMeta{Name: testNodeName},
		Spec: nncv1.NodeNetworkConfigSpec{
			Allocations: []nncv1.Allocation{{Network: "default", Pods: 16}},
			ReleasableCIDRs: []nncv1.PodCIDR{
				{Id: released, Network: "default", CIDR: released},
			},
		},
		Status: nncv1.NodeNetworkConfigStatus{
			PodCIDRs: []nncv1.PodCIDR{
				{Id: "10.100.0.0/28", Network: "default", CIDR: "10.100.0.0/28"},
				{Id: released, Network: "default", CIDR: released},
			},
		},
	}
	if _, err := f.nncClient.NetworkingV1().NodeNetworkConfigs().Create(ctx, nnc, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create NodeNetworkConfig: %v", err)
	}
	f.informerFactory.WaitForCacheSync(stopCh)

	gceWrites := countGCEWrites(t, f)

	if err := f.reconcile(ctx, nnc, testProviderID); err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	if got := releasableCIDRsOf(ctx, t, f); len(got) != 0 {
		t.Errorf("Expected the stale entry to be pruned after restart, got %v", got)
	}
	if *gceWrites != 0 {
		t.Errorf("Expected no GCE writes during crash recovery, got %d", *gceWrites)
	}

	// The status controller should also have reconciled status back to GCE truth.
	final, err := f.nncClient.NetworkingV1().NodeNetworkConfigs().Get(ctx, testNodeName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get NodeNetworkConfig: %v", err)
	}
	if len(final.Status.PodCIDRs) != 1 {
		t.Errorf("Expected status to converge to 1 PodCIDR, got %d: %v", len(final.Status.PodCIDRs), final.Status.PodCIDRs)
	}
}

// TestPruneReleasableCIDRs_OrphanedEntry covers an entry naming a range that was
// never on this node, which is also the recovery path for entries the daemon
// stranded and can no longer clean up itself.
func TestPruneReleasableCIDRs_OrphanedEntry(t *testing.T) {
	ctx := context.Background()
	f := newTestFixture(t)
	stopCh := make(chan struct{})
	defer close(stopCh)
	f.run(ctx, stopCh)

	insertInstanceWithRanges(ctx, t, f, "10.100.0.0/28")

	nnc := &nncv1.NodeNetworkConfig{
		ObjectMeta: metav1.ObjectMeta{Name: testNodeName},
		Spec: nncv1.NodeNetworkConfigSpec{
			Allocations: []nncv1.Allocation{{Network: "default", Pods: 16}},
			ReleasableCIDRs: []nncv1.PodCIDR{
				{Id: "192.168.7.0/28", Network: "default", CIDR: "192.168.7.0/28"},
			},
		},
	}
	if _, err := f.nncClient.NetworkingV1().NodeNetworkConfigs().Create(ctx, nnc, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create NodeNetworkConfig: %v", err)
	}
	f.informerFactory.WaitForCacheSync(stopCh)

	gceWrites := countGCEWrites(t, f)

	if err := f.reconcile(ctx, nnc, testProviderID); err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	if got := releasableCIDRsOf(ctx, t, f); len(got) != 0 {
		t.Errorf("Expected the orphaned entry to be pruned, got %v", got)
	}
	if *gceWrites != 0 {
		t.Errorf("Expected no GCE writes for an orphaned entry, got %d", *gceWrites)
	}
}

// TestPruneReleasableCIDRs_NoWriteWhenNothingStale guards against a spec write
// on every reconcile, which would be a hot loop in production.
func TestPruneReleasableCIDRs_NoWriteWhenNothingStale(t *testing.T) {
	ctx := context.Background()
	f := newTestFixture(t)
	stopCh := make(chan struct{})
	defer close(stopCh)
	f.run(ctx, stopCh)

	insertInstanceWithRanges(ctx, t, f, "10.100.0.0/28")

	nnc := &nncv1.NodeNetworkConfig{
		ObjectMeta: metav1.ObjectMeta{Name: testNodeName},
		Spec: nncv1.NodeNetworkConfigSpec{
			Allocations: []nncv1.Allocation{{Network: "default", Pods: 16}},
		},
	}
	if _, err := f.nncClient.NetworkingV1().NodeNetworkConfigs().Create(ctx, nnc, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create NodeNetworkConfig: %v", err)
	}
	f.informerFactory.WaitForCacheSync(stopCh)

	// Count writes to the main resource only; the status subresource is a
	// separate endpoint and is expected to be written.
	specWrites := 0
	f.nncClient.PrependReactor("update", "nodenetworkconfigs", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		if action.GetSubresource() == "" {
			specWrites++
		}
		return false, nil, nil
	})

	if err := f.reconcile(ctx, nnc, testProviderID); err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}
	if specWrites != 0 {
		t.Errorf("Expected no spec writes when nothing is stale, got %d", specWrites)
	}
}

// TestPruneReleasableCIDRs_RetriesOnConflict verifies that a 409 is retried
// against fresh state, and that the retry does not clobber a concurrent
// Allocations edit made by the daemon.
func TestPruneReleasableCIDRs_RetriesOnConflict(t *testing.T) {
	ctx := context.Background()
	f := newTestFixture(t)
	stopCh := make(chan struct{})
	defer close(stopCh)
	f.run(ctx, stopCh)

	const released = "10.100.1.0/28"
	insertInstanceWithRanges(ctx, t, f, "10.100.0.0/28")

	nnc := &nncv1.NodeNetworkConfig{
		ObjectMeta: metav1.ObjectMeta{Name: testNodeName},
		Spec: nncv1.NodeNetworkConfigSpec{
			Allocations: []nncv1.Allocation{{Network: "default", Pods: 16}},
			ReleasableCIDRs: []nncv1.PodCIDR{
				{Id: released, Network: "default", CIDR: released},
			},
		},
	}
	if _, err := f.nncClient.NetworkingV1().NodeNetworkConfigs().Create(ctx, nnc, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create NodeNetworkConfig: %v", err)
	}
	f.informerFactory.WaitForCacheSync(stopCh)

	// Fail the first spec write with a conflict, and concurrently bump
	// Allocations the way the daemon would, so the retry has to pick up the new
	// value rather than writing back the copy it already had.
	//
	// The concurrent edit goes through the tracker, not the client: Fake.Invokes
	// holds the clientset lock for the duration of this callback, so a nested
	// client call would deadlock against it.
	specWrites := 0
	f.nncClient.PrependReactor("update", "nodenetworkconfigs", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		if action.GetSubresource() != "" {
			return false, nil, nil
		}
		specWrites++
		if specWrites != 1 {
			return false, nil, nil
		}

		gvr := action.GetResource()
		obj, err := f.nncClient.Tracker().Get(gvr, action.GetNamespace(), testNodeName)
		if err != nil {
			return true, nil, err
		}
		live, ok := obj.(*nncv1.NodeNetworkConfig)
		if !ok {
			return true, nil, fmt.Errorf("unexpected object type %T in tracker", obj)
		}
		live = live.DeepCopy()
		live.Spec.Allocations[0].Pods = 48
		if err := f.nncClient.Tracker().Update(gvr, live, action.GetNamespace()); err != nil {
			return true, nil, err
		}
		return true, nil, apierrors.NewConflict(
			schema.GroupResource{Group: "networking.gke.io", Resource: "nodenetworkconfigs"},
			testNodeName, fmt.Errorf("simulated conflict"))
	})

	if err := f.specCtrl.reconcile(ctx, nnc, testProviderID); err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	if specWrites < 2 {
		t.Errorf("Expected the conflicting write to be retried, saw %d spec writes", specWrites)
	}
	if got := releasableCIDRsOf(ctx, t, f); len(got) != 0 {
		t.Errorf("Expected the entry to be pruned after the retry, got %v", got)
	}

	final, err := f.nncClient.NetworkingV1().NodeNetworkConfigs().Get(ctx, testNodeName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get NodeNetworkConfig: %v", err)
	}
	if final.Spec.Allocations[0].Pods != 48 {
		t.Errorf("Retry clobbered a concurrent Allocations edit: expected Pods=48, got %d", final.Spec.Allocations[0].Pods)
	}
}

// TestCalculateChanges_ReleasableExcludedFromCapacity covers the under-allocation
// bug: a scale-up that coincides with a release must be sized against capacity
// that excludes the range being released.
func TestCalculateChanges_ReleasableExcludedFromCapacity(t *testing.T) {
	ctx := context.Background()
	f := newTestFixture(t)
	stopCh := make(chan struct{})
	defer close(stopCh)
	f.run(ctx, stopCh)

	const (
		keep     = "10.100.0.0/28"
		released = "10.100.1.0/28"
	)
	insertInstanceWithRanges(ctx, t, f, keep, released)

	// GCE holds 32 IPs. The daemon is releasing 16 of them and, having already
	// netted that out of its own ask, requests 32. Counting the released range
	// would make this look satisfied and leave the node at 16.
	nnc := &nncv1.NodeNetworkConfig{
		ObjectMeta: metav1.ObjectMeta{Name: testNodeName},
		Spec: nncv1.NodeNetworkConfigSpec{
			Allocations: []nncv1.Allocation{{Network: "default", Pods: 32}},
			ReleasableCIDRs: []nncv1.PodCIDR{
				{Id: released, Network: "default", CIDR: released},
			},
		},
	}

	ifaces, err := f.specCtrl.gceCache.Get(ctx, testNodeName, testProviderID)
	if err != nil {
		t.Fatalf("Failed to read GCE state: %v", err)
	}
	changes, err := f.specCtrl.calculateChanges(nnc, ifaces)
	if err != nil {
		t.Fatalf("calculateChanges failed: %v", err)
	}

	netChanges := changes.GetNetwork("default")
	// Usable capacity is 16 against a desire of 32, so exactly one more block.
	if len(netChanges.additions) != 1 {
		t.Errorf("Expected 1 addition once the releasable range is excluded from capacity, got %d: %v",
			len(netChanges.additions), netChanges.additions)
	}
	if len(netChanges.removals) != 1 || netChanges.removals[0] != released {
		t.Errorf("Expected exactly the released range to be removed, got %v", netChanges.removals)
	}
}

// cacheHasNode reports whether the cache still holds an entry for nodeName.
func cacheHasNode(c *GCECache, nodeName string) bool {
	c.mapLock.Lock()
	defer c.mapLock.Unlock()

	_, ok := c.entries[nodeName]
	return ok
}

// newCountingLoader returns a GCEInstanceLoader and a map recording how many times each
// providerID was loaded from GCE.
func newCountingLoader() (GCEInstanceLoader, map[string]int) {
	loads := map[string]int{}
	loader := func(ctx context.Context, providerID string) ([]*networkInterface, error) {
		loads[providerID]++
		return []*networkInterface{
			{
				Name:          "nic0",
				Network:       testNetworkURL,
				AliasIPRanges: []string{"10.100.0.0/28"},
			},
		}, nil
	}
	return loader, loads
}

// TestGCECache_AgeCleanUpReclaimsIdleEntries verifies that an entry which is never looked up again
// (the deleted-node case) is reclaimed by activity on other nodes, rather than lingering until the
// cache hits its entry cap.
func TestGCECache_AgeCleanUpReclaimsIdleEntries(t *testing.T) {
	ctx := context.Background()
	loader, loads := newCountingLoader()
	fakeClock := clocktesting.NewFakeClock(time.Date(2026, 6, 26, 9, 0, 0, 0, time.UTC))

	// 10s freshness TTL, 10m max observation age, ample capacity.
	c := NewGCECacheWithLimits(loader, 10*time.Second, 10*time.Minute, DefaultCacheMaxEntries, fakeClock)

	// "deleted-node" is observed once and then never reconciled again.
	if _, err := c.Get(ctx, "deleted-node", "deleted-node"); err != nil {
		t.Fatalf("Get(deleted-node) failed: %v", err)
	}
	if loads["deleted-node"] != 1 {
		t.Fatalf("Expected 1 GCE load, got %d", loads["deleted-node"])
	}
	if !cacheHasNode(c, "deleted-node") {
		t.Fatalf("Expected deleted-node to be cached immediately after Get")
	}

	// Still within maxAge: unrelated traffic must not evict it.
	fakeClock.Step(5 * time.Minute)
	if _, err := c.Get(ctx, "live-node", "live-node"); err != nil {
		t.Fatalf("Get(live-node) failed: %v", err)
	}
	if !cacheHasNode(c, "deleted-node") {
		t.Errorf("Expected deleted-node to still be cached 5m after its last observation")
	}

	// Past maxAge: the next lookup of any node sweeps it out, even though nothing ever asks
	// for deleted-node again and the cache is nowhere near its entry cap.
	fakeClock.Step(5*time.Minute + time.Second)
	if _, err := c.Get(ctx, "live-node", "live-node"); err != nil {
		t.Fatalf("Get(live-node) failed: %v", err)
	}
	if cacheHasNode(c, "deleted-node") {
		t.Errorf("Expected deleted-node to be evicted 10m after its last observation")
	}

	// The sweep must stop at the first live entry: live-node was refreshed more recently and
	// has to survive.
	if !cacheHasNode(c, "live-node") {
		t.Errorf("Expected live-node to survive the age sweep")
	}

	// Re-reading the evicted node repopulates it from GCE.
	if _, err := c.Get(ctx, "deleted-node", "deleted-node"); err != nil {
		t.Fatalf("Get(deleted-node) failed: %v", err)
	}
	if loads["deleted-node"] != 2 {
		t.Errorf("Expected 2 GCE loads after eviction, got %d", loads["deleted-node"])
	}
}

func TestGCECache_RefreshExtendsMaxAge(t *testing.T) {
	ctx := context.Background()
	loader, loads := newCountingLoader()
	fakeClock := clocktesting.NewFakeClock(time.Date(2026, 6, 26, 9, 0, 0, 0, time.UTC))

	c := NewGCECacheWithLimits(loader, 10*time.Second, 10*time.Minute, DefaultCacheMaxEntries, fakeClock)

	if _, err := c.Get(ctx, testNodeName, testProviderID); err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	// Refresh the observation just before the entry would have aged out.
	fakeClock.Step(9 * time.Minute)
	if _, err := c.Get(ctx, testNodeName, testProviderID); err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if loads[testProviderID] != 2 {
		t.Fatalf("Expected the 9m-old observation to be refreshed, got %d loads", loads[testProviderID])
	}

	// maxAge is measured from the refreshed observation, not from entry creation, so the entry
	// survives past its original deadline even once a sweep runs.
	fakeClock.Step(9 * time.Minute)
	if _, err := c.Get(ctx, "other-node", "other-node"); err != nil {
		t.Fatalf("Get(other-node) failed: %v", err)
	}
	if !cacheHasNode(c, testNodeName) {
		t.Errorf("Expected node %q to still be cached 9m after a refresh", testNodeName)
	}
}

func TestGCECache_EvictsLeastRecentlyUsedWhenFull(t *testing.T) {
	ctx := context.Background()
	loader, loads := newCountingLoader()
	fakeClock := clocktesting.NewFakeClock(time.Date(2026, 6, 26, 9, 0, 0, 0, time.UTC))

	// Cap the cache at two nodes so eviction is observable.
	c := NewGCECacheWithLimits(loader, 10*time.Second, 10*time.Minute, 2, fakeClock)

	for _, node := range []string{"node-a", "node-b"} {
		if _, err := c.Get(ctx, node, node); err != nil {
			t.Fatalf("Get(%q) failed: %v", node, err)
		}
	}

	// Touch node-a so that node-b becomes the least recently used entry.
	if _, err := c.Get(ctx, "node-a", "node-a"); err != nil {
		t.Fatalf("Get(node-a) failed: %v", err)
	}
	if loads["node-a"] != 1 {
		t.Fatalf("Expected the fresh node-a observation to be reused, got %d loads", loads["node-a"])
	}

	// Adding a third node exceeds the cap and must evict node-b, not node-a.
	if _, err := c.Get(ctx, "node-c", "node-c"); err != nil {
		t.Fatalf("Get(node-c) failed: %v", err)
	}
	if !cacheHasNode(c, "node-a") {
		t.Errorf("Expected node-a to be retained as the most recently used entry")
	}
	if !cacheHasNode(c, "node-c") {
		t.Errorf("Expected node-c to be cached")
	}
	if cacheHasNode(c, "node-b") {
		t.Errorf("Expected node-b to be evicted as the least recently used entry")
	}

	// node-b's observation is gone, so reading it again hits GCE.
	if _, err := c.Get(ctx, "node-b", "node-b"); err != nil {
		t.Fatalf("Get(node-b) failed: %v", err)
	}
	if loads["node-b"] != 2 {
		t.Errorf("Expected node-b to be reloaded from GCE after eviction, got %d loads", loads["node-b"])
	}
}

// TestGCECache_ConcurrentAccessRespectsBounds hammers the cache from several goroutines over more
// distinct nodes than it can hold, so that lookups, refreshes and capacity evictions interleave.
// Under -race this also covers the lock ordering between mapLock and the per-node mutexes.
func TestGCECache_ConcurrentAccessRespectsBounds(t *testing.T) {
	ctx := context.Background()

	var loaderMu sync.Mutex
	loads := 0
	loader := func(ctx context.Context, providerID string) ([]*networkInterface, error) {
		loaderMu.Lock()
		defer loaderMu.Unlock()
		loads++
		return []*networkInterface{
			{
				Name:          "nic0",
				Network:       testNetworkURL,
				AliasIPRanges: []string{"10.100.0.0/28"},
			},
		}, nil
	}

	fakeClock := clocktesting.NewFakeClock(time.Date(2026, 6, 26, 9, 0, 0, 0, time.UTC))
	const maxEntries = 8
	c := NewGCECacheWithLimits(loader, 10*time.Second, 10*time.Minute, maxEntries, fakeClock)

	var wg sync.WaitGroup
	const (
		workers          = 16
		getsPerWorker    = 50
		distinctNodes    = 32
		expectedGCELoads = workers * getsPerWorker
	)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for j := 0; j < getsPerWorker; j++ {
				node := fmt.Sprintf("node-%d", (worker+j)%distinctNodes)
				if _, err := c.ForceGet(ctx, node, node); err != nil {
					t.Errorf("ForceGet(%q) failed: %v", node, err)
					return
				}
			}
		}(i)
	}
	wg.Wait()

	// ForceGet always bypasses the freshness TTL, so every call must reach the loader.
	loaderMu.Lock()
	if loads != expectedGCELoads {
		t.Errorf("Expected %d GCE loads, got %d", expectedGCELoads, loads)
	}
	loaderMu.Unlock()

	c.mapLock.Lock()
	defer c.mapLock.Unlock()

	if got := len(c.entries); got > maxEntries {
		t.Errorf("Expected at most %d cached entries, got %d", maxEntries, got)
	}
	if got := c.lruList.Len(); got != len(c.entries) {
		t.Errorf("lruList has %d elements but the entries map has %d", got, len(c.entries))
	}
	if got := c.ageList.Len(); got != len(c.entries) {
		t.Errorf("ageList has %d elements but the entries map has %d", got, len(c.entries))
	}
}

// ---------------------------------------------------------------------------
// Model-based randomized eviction test.
//
// GCECache evicts incrementally: it walks a prefix of a list it believes is sorted by expiry, and
// pops the back of a list it believes is ordered by recency. Those are easy to get subtly wrong.
// cacheModel below reimplements the same policy in the dumbest possible way -- a full rescan of a
// plain map on every operation -- so that a randomized sequence of operations can be replayed
// against both and compared after every step.
// ---------------------------------------------------------------------------

// modelEntry mirrors the bookkeeping GCECache keeps for a single node.
type modelEntry struct {
	expiresAt   time.Time
	lastUpdated time.Time
	// usedAt is a logical timestamp; the entry with the lowest value is the least recently used.
	usedAt uint64
}

// cacheModel is a deliberately naive reference implementation of GCECache's eviction policy.
type cacheModel struct {
	entries    map[string]*modelEntry
	ttl        time.Duration
	maxAge     time.Duration
	maxEntries int
	seq        uint64
}

func newCacheModel(ttl, maxAge time.Duration, maxEntries int) *cacheModel {
	return &cacheModel{
		entries:    map[string]*modelEntry{},
		ttl:        ttl,
		maxAge:     maxAge,
		maxEntries: maxEntries,
	}
}

// touch marks an entry as the most recently used one.
func (m *cacheModel) touch(e *modelEntry) {
	m.seq++
	e.usedAt = m.seq
}

// evictLRU removes the entry with the lowest usedAt by scanning every entry.
func (m *cacheModel) evictLRU() {
	victim := ""
	var lowest uint64
	for name, e := range m.entries {
		if victim == "" || e.usedAt < lowest {
			victim, lowest = name, e.usedAt
		}
	}
	if victim != "" {
		delete(m.entries, victim)
	}
}

// get replays GCECache.get against the model. loadFails reports whether the GCE loader would have
// returned an error on this call.
func (m *cacheModel) get(now time.Time, node string, force, loadFails bool) {
	// Age cleanup, by brute-force scan rather than by walking a sorted prefix.
	for name, e := range m.entries {
		if !now.Before(e.expiresAt) {
			delete(m.entries, name)
		}
	}

	e, ok := m.entries[node]
	if ok {
		m.touch(e)
	} else {
		for len(m.entries) >= m.maxEntries {
			m.evictLRU()
		}
		e = &modelEntry{expiresAt: now.Add(m.maxAge)}
		m.entries[node] = e
		m.touch(e)
	}

	// Refresh the observation if it is missing, stale, or the caller forced it.
	if force || e.lastUpdated.IsZero() || now.Sub(e.lastUpdated) > m.ttl {
		if loadFails {
			// A failed load leaves the entry in place but does not refresh it.
			return
		}
		e.lastUpdated = now
		e.expiresAt = now.Add(m.maxAge)
		m.touch(e)
	}
}

// cacheSnapshot is a copy of a GCECache's internal state, taken under its lock.
type cacheSnapshot struct {
	expiry    map[string]time.Time
	lruOrder  []string // most to least recently used
	ageOrder  []string // soonest to latest expiry
	ageExpiry []time.Time
}

func snapshotCache(c *GCECache) cacheSnapshot {
	c.mapLock.Lock()
	defer c.mapLock.Unlock()

	s := cacheSnapshot{expiry: map[string]time.Time{}}
	for name, e := range c.entries {
		s.expiry[name] = e.expiresAt
	}
	for e := c.lruList.Front(); e != nil; e = e.Next() {
		s.lruOrder = append(s.lruOrder, e.Value.(*cacheEntry).nodeName)
	}
	for e := c.ageList.Front(); e != nil; e = e.Next() {
		entry := e.Value.(*cacheEntry)
		s.ageOrder = append(s.ageOrder, entry.nodeName)
		s.ageExpiry = append(s.ageExpiry, entry.expiresAt)
	}
	return s
}

func sortedNames(m map[string]*modelEntry) []string {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedExpiryNames(m map[string]time.Time) []string {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// compareToModel returns a description of the first discrepancy found between the cache and the
// model, or "" if they agree and all of the cache's internal invariants hold.
func compareToModel(c *GCECache, m *cacheModel) string {
	s := snapshotCache(c)

	// The contents must match the naive model exactly. This is what catches an age sweep that
	// stops too early or an LRU eviction that picks the wrong victim.
	if len(s.expiry) != len(m.entries) {
		return fmt.Sprintf("cache holds %v, model holds %v", sortedExpiryNames(s.expiry), sortedNames(m.entries))
	}
	for name, expiresAt := range s.expiry {
		me, ok := m.entries[name]
		if !ok {
			return fmt.Sprintf("cache holds unexpected entry %q; model holds %v", name, sortedNames(m.entries))
		}
		if !me.expiresAt.Equal(expiresAt) {
			return fmt.Sprintf("entry %q expires at %v, model expects %v", name, expiresAt, me.expiresAt)
		}
	}

	// Hard bound on memory.
	if len(s.expiry) > m.maxEntries {
		return fmt.Sprintf("cache holds %d entries, exceeding maxEntries=%d", len(s.expiry), m.maxEntries)
	}

	// The map and both ordering lists must stay in lockstep; a leaked list element would mean a
	// slow memory leak that the contents check above cannot see.
	if len(s.lruOrder) != len(s.expiry) {
		return fmt.Sprintf("lruList has %d elements but the entries map has %d", len(s.lruOrder), len(s.expiry))
	}
	if len(s.ageOrder) != len(s.expiry) {
		return fmt.Sprintf("ageList has %d elements but the entries map has %d", len(s.ageOrder), len(s.expiry))
	}

	// ageCleanUp stops at the first live entry, so this ordering is load-bearing: if it ever
	// breaks, expired entries silently survive.
	for i := 1; i < len(s.ageExpiry); i++ {
		if s.ageExpiry[i].Before(s.ageExpiry[i-1]) {
			return fmt.Sprintf("ageList is not sorted by expiry: %q expires at %v but follows %q expiring at %v",
				s.ageOrder[i], s.ageExpiry[i], s.ageOrder[i-1], s.ageExpiry[i-1])
		}
	}

	// Capacity eviction pops the back of lruList, so it must really be ordered by recency.
	for i := 1; i < len(s.lruOrder); i++ {
		prev, cur := m.entries[s.lruOrder[i-1]], m.entries[s.lruOrder[i]]
		if prev.usedAt < cur.usedAt {
			return fmt.Sprintf("lruList is not ordered by recency: %q (used at %d) precedes %q (used at %d)",
				s.lruOrder[i-1], prev.usedAt, s.lruOrder[i], cur.usedAt)
		}
	}

	return ""
}

// TestGCECache_EvictionMatchesModel replays pseudorandom operation sequences against both GCECache
// and a naive reference model, comparing them after every operation.
//
// Seeds are fixed so failures are reproducible, and the whole test is bounded to a few tens of
// thousands of operations over caches holding at most a handful of entries, so it runs in well
// under a second.
func TestGCECache_EvictionMatchesModel(t *testing.T) {
	const (
		seeds        = 50
		opsPerSeed   = 500
		nodeUniverse = 12
	)

	ctx := context.Background()
	start := time.Date(2026, 6, 26, 9, 0, 0, 0, time.UTC)

	for seed := int64(0); seed < seeds; seed++ {
		rng := rand.New(rand.NewSource(seed))

		// Keep maxEntries well below nodeUniverse so that capacity eviction fires constantly,
		// and maxAge within a few clock steps so that age eviction does too.
		maxEntries := 1 + rng.Intn(6)
		maxAge := time.Duration(1+rng.Intn(8)) * time.Second
		ttl := time.Duration(rng.Intn(4)) * time.Second

		fakeClock := clocktesting.NewFakeClock(start)

		loadFails := false
		loader := func(ctx context.Context, providerID string) ([]*networkInterface, error) {
			if loadFails {
				return nil, fmt.Errorf("simulated GCE failure for %q", providerID)
			}
			return []*networkInterface{
				{
					Name:          "nic0",
					Network:       testNetworkURL,
					AliasIPRanges: []string{"10.100.0.0/28"},
				},
			}, nil
		}

		c := NewGCECacheWithLimits(loader, ttl, maxAge, maxEntries, fakeClock)
		model := newCacheModel(ttl, maxAge, maxEntries)

		for op := 0; op < opsPerSeed; op++ {
			switch {
			case rng.Intn(20) == 0:
				// Occasionally jump far enough to age out the entire cache.
				fakeClock.Step(2 * maxAge)
			default:
				// Otherwise drift by 0-3s, so entries expire at staggered times.
				fakeClock.Step(time.Duration(rng.Intn(4)) * time.Second)
			}

			node := fmt.Sprintf("node-%d", rng.Intn(nodeUniverse))
			force := rng.Intn(4) == 0
			loadFails = rng.Intn(8) == 0

			// The fake clock only moves when stepped, so the cache observes this same instant.
			now := fakeClock.Now()

			var err error
			if force {
				_, err = c.ForceGet(ctx, node, node)
			} else {
				_, err = c.Get(ctx, node, node)
			}
			if err != nil && !loadFails {
				t.Fatalf("seed %d op %d: unexpected error from Get(%q): %v", seed, op, node, err)
			}

			model.get(now, node, force, loadFails)

			if msg := compareToModel(c, model); msg != "" {
				t.Fatalf("seed %d op %d (node=%q force=%v loadFails=%v maxEntries=%d maxAge=%v ttl=%v): %s",
					seed, op, node, force, loadFails, maxEntries, maxAge, ttl, msg)
			}
		}
	}
}

// TestSetNNCCondition_TransitionTimeFollowsConvention pins the API convention
// that LastTransitionTime marks when Status last changed, not when the
// condition was last written. Alerting that measures "how long has this node
// been unready" reads that field, so a reworded message must not reset it.
func TestSetNNCCondition_TransitionTimeFollowsConvention(t *testing.T) {
	original := metav1.NewTime(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	condType := string(nncv1.NodeNetworkConfigConditionReady)

	nnc := &nncv1.NodeNetworkConfig{
		ObjectMeta: metav1.ObjectMeta{Name: testNodeName},
		Status: nncv1.NodeNetworkConfigStatus{
			Conditions: []metav1.Condition{
				{
					Type:               condType,
					Status:             metav1.ConditionFalse,
					LastTransitionTime: original,
					Reason:             "Updating",
					Message:            "first message",
				},
			},
		},
	}

	// Same status, different reason and message: the condition changes, but the
	// transition time must be preserved.
	if changed := setNNCCondition(nnc, condType, metav1.ConditionFalse, "StillUpdating", "second message"); !changed {
		t.Errorf("Expected setNNCCondition to report a change when reason and message differ")
	}
	got := getCondition(nnc.Status.Conditions, condType)
	if got == nil {
		t.Fatalf("Condition %q disappeared", condType)
	}
	if !got.LastTransitionTime.Equal(&original) {
		t.Errorf("LastTransitionTime was restamped on a message-only change: got %v, want %v", got.LastTransitionTime, original)
	}
	if got.Reason != "StillUpdating" || got.Message != "second message" {
		t.Errorf("Reason/Message were not updated: got %q/%q", got.Reason, got.Message)
	}

	// Identical write: nothing changed at all.
	if changed := setNNCCondition(nnc, condType, metav1.ConditionFalse, "StillUpdating", "second message"); changed {
		t.Errorf("Expected setNNCCondition to report no change for an identical write")
	}

	// Real transition: the timestamp must move.
	if changed := setNNCCondition(nnc, condType, metav1.ConditionTrue, "Ready", "all good"); !changed {
		t.Errorf("Expected setNNCCondition to report a change on a status transition")
	}
	got = getCondition(nnc.Status.Conditions, condType)
	if got.LastTransitionTime.Equal(&original) {
		t.Errorf("LastTransitionTime was not updated on a real status transition")
	}
	if got.Status != metav1.ConditionTrue {
		t.Errorf("Expected status True, got %v", got.Status)
	}
}

// TestSyncStatusToGCE_PreservesPodCIDRTransitionTime covers the same convention
// for the per-range conditions. Status.PodCIDRs is rebuilt wholesale on every
// sync, so adding one range must not restamp the ranges that were already
// published and unchanged.
func TestSyncStatusToGCE_PreservesPodCIDRTransitionTime(t *testing.T) {
	ctx := context.Background()
	f := newTestFixture(t)
	stopCh := make(chan struct{})
	defer close(stopCh)
	f.run(ctx, stopCh)

	const (
		firstCIDR  = "10.100.0.0/28"
		secondCIDR = "10.100.1.0/28"
	)
	instanceKey := insertInstanceWithRanges(ctx, t, f, firstCIDR)

	nnc := &nncv1.NodeNetworkConfig{
		ObjectMeta: metav1.ObjectMeta{Name: testNodeName},
		Spec: nncv1.NodeNetworkConfigSpec{
			Allocations: []nncv1.Allocation{{Network: "default", Pods: 16}},
		},
	}
	if _, err := f.nncClient.NetworkingV1().NodeNetworkConfigs().Create(ctx, nnc, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create NodeNetworkConfig: %v", err)
	}
	f.informerFactory.WaitForCacheSync(stopCh)

	if err := f.statusCtrl.reconcile(ctx, nnc, testProviderID); err != nil {
		t.Fatalf("Initial status reconcile failed: %v", err)
	}

	// Backdate the published condition to a known instant, so the assertion does
	// not depend on the clock advancing between the two syncs.
	backdated := metav1.NewTime(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	current, err := f.nncClient.NetworkingV1().NodeNetworkConfigs().Get(ctx, testNodeName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get NodeNetworkConfig: %v", err)
	}
	if len(current.Status.PodCIDRs) != 1 {
		t.Fatalf("Expected 1 published pod CIDR, got %d", len(current.Status.PodCIDRs))
	}
	current.Status.PodCIDRs[0].Condition.LastTransitionTime = backdated
	if _, err := f.nncClient.NetworkingV1().NodeNetworkConfigs().UpdateStatus(ctx, current, metav1.UpdateOptions{}); err != nil {
		t.Fatalf("Failed to backdate condition: %v", err)
	}

	// Attach a second range and resync.
	instance, err := f.fakeGCE.Compute().BetaInstances().Get(ctx, instanceKey)
	if err != nil {
		t.Fatalf("Failed to get GCE instance: %v", err)
	}
	instance.NetworkInterfaces[0].AliasIpRanges = append(
		instance.NetworkInterfaces[0].AliasIpRanges,
		&computebeta.AliasIpRange{IpCidrRange: secondCIDR},
	)
	mockInstances, ok := f.fakeGCE.Compute().BetaInstances().(*gcloud.MockBetaInstances)
	if !ok {
		t.Fatalf("Failed to cast BetaInstances to MockBetaInstances")
	}
	mockInstances.Objects[*instanceKey] = &gcloud.MockInstancesObj{Obj: instance}

	current, err = f.nncClient.NetworkingV1().NodeNetworkConfigs().Get(ctx, testNodeName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get NodeNetworkConfig: %v", err)
	}
	if err := f.statusCtrl.reconcile(ctx, current, testProviderID); err != nil {
		t.Fatalf("Second status reconcile failed: %v", err)
	}

	final, err := f.nncClient.NetworkingV1().NodeNetworkConfigs().Get(ctx, testNodeName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get NodeNetworkConfig: %v", err)
	}
	if len(final.Status.PodCIDRs) != 2 {
		t.Fatalf("Expected 2 published pod CIDRs, got %d", len(final.Status.PodCIDRs))
	}

	for _, pc := range final.Status.PodCIDRs {
		switch pc.CIDR {
		case firstCIDR:
			if !pc.Condition.LastTransitionTime.Equal(&backdated) {
				t.Errorf("Unchanged range %s was restamped: got %v, want %v",
					pc.CIDR, pc.Condition.LastTransitionTime, backdated)
			}
		case secondCIDR:
			if pc.Condition.LastTransitionTime.Equal(&backdated) {
				t.Errorf("Newly added range %s should have a fresh transition time", pc.CIDR)
			}
		default:
			t.Errorf("Unexpected pod CIDR %s", pc.CIDR)
		}
	}
}

// TestSyncStatusToGCE_RestampsOnRealTransition is the other half of the
// convention: preserving the timestamp is only correct while the status is
// unchanged. A range published as not-Ready that becomes Ready has genuinely
// transitioned and must be restamped.
func TestSyncStatusToGCE_RestampsOnRealTransition(t *testing.T) {
	ctx := context.Background()
	f := newTestFixture(t)
	stopCh := make(chan struct{})
	defer close(stopCh)
	f.run(ctx, stopCh)

	const (
		notRoutableCIDR = "10.100.0.0/28"
		addedCIDR       = "10.100.1.0/28"
	)
	// Both ranges exist in GCE, but status only knows about the first, so the
	// CIDR sets differ and a resync is triggered.
	insertInstanceWithRanges(ctx, t, f, notRoutableCIDR, addedCIDR)

	backdated := metav1.NewTime(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))
	nnc := &nncv1.NodeNetworkConfig{
		ObjectMeta: metav1.ObjectMeta{Name: testNodeName},
		Spec: nncv1.NodeNetworkConfigSpec{
			Allocations: []nncv1.Allocation{{Network: "default", Pods: 32}},
		},
		Status: nncv1.NodeNetworkConfigStatus{
			PodCIDRs: []nncv1.PodCIDR{
				{
					Id:      notRoutableCIDR,
					Network: "default",
					CIDR:    notRoutableCIDR,
					Condition: &metav1.Condition{
						Type:               string(nncv1.PodCIDRConditionReady),
						Status:             metav1.ConditionFalse,
						LastTransitionTime: backdated,
						Reason:             string(nncv1.PodCIDRReadyConditionNotRoutable),
						Message:            "Pod CIDR is not routable",
					},
				},
			},
		},
	}
	if _, err := f.nncClient.NetworkingV1().NodeNetworkConfigs().Create(ctx, nnc, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create NodeNetworkConfig: %v", err)
	}
	f.informerFactory.WaitForCacheSync(stopCh)

	if err := f.statusCtrl.reconcile(ctx, nnc, testProviderID); err != nil {
		t.Fatalf("Status reconcile failed: %v", err)
	}

	final, err := f.nncClient.NetworkingV1().NodeNetworkConfigs().Get(ctx, testNodeName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get NodeNetworkConfig: %v", err)
	}
	if len(final.Status.PodCIDRs) != 2 {
		t.Fatalf("Expected 2 published pod CIDRs, got %d", len(final.Status.PodCIDRs))
	}
	for _, pc := range final.Status.PodCIDRs {
		if pc.Condition.Status != metav1.ConditionTrue {
			t.Errorf("Range %s: expected Ready/True, got %v", pc.CIDR, pc.Condition.Status)
		}
		if pc.Condition.LastTransitionTime.Equal(&backdated) {
			t.Errorf("Range %s transitioned to Ready but kept its stale transition time %v",
				pc.CIDR, backdated)
		}
	}
}

// nodeWithProviderID builds a Node carrying the given provider ID.
func nodeWithProviderID(providerID string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: testNodeName},
		Spec:       corev1.NodeSpec{ProviderID: providerID},
	}
}

// TestStatusController_WatchesNodes covers the reason this handler exists: in a
// multi-networking cluster the spec controller is not running, so nothing else
// would ever enqueue a node. A Node appearing must be enough on its own.
func TestStatusController_WatchesNodes(t *testing.T) {
	ctx := context.Background()
	f := newTestFixture(t)
	stopCh := make(chan struct{})
	defer close(stopCh)

	if _, err := f.kubeClient.CoreV1().Nodes().Create(ctx, nodeWithProviderID(testProviderID), metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create Node: %v", err)
	}
	f.runNodeInformer(stopCh)

	// Informer delivery is asynchronous; poll briefly rather than sleeping.
	deadline := time.Now().Add(5 * time.Second)
	for f.statusCtrl.queue.Len() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := f.statusCtrl.queue.Len(); got != 1 {
		t.Errorf("Expected the Node add to enqueue exactly one key, queue length is %d", got)
	}
}

// TestStatusController_NodeUpdateFilter pins the filter. Every enqueue costs an
// uncached GCE instances.get, so a routine kubelet write must not produce one,
// while anything that could change the published ranges must.
func TestStatusController_NodeUpdateFilter(t *testing.T) {
	withAnnotations := func(n *corev1.Node, annotations map[string]string) *corev1.Node {
		n = n.DeepCopy()
		n.Annotations = annotations
		return n
	}
	withCapacity := func(n *corev1.Node, capacity corev1.ResourceList) *corev1.Node {
		n = n.DeepCopy()
		n.Status.Capacity = capacity
		return n
	}
	withHeartbeat := func(n *corev1.Node) *corev1.Node {
		n = n.DeepCopy()
		n.Status.Conditions = []corev1.NodeCondition{{
			Type:              corev1.NodeReady,
			Status:            corev1.ConditionTrue,
			LastHeartbeatTime: metav1.Now(),
		}}
		n.ResourceVersion = "99"
		return n
	}

	ready := nodeWithProviderID(testProviderID)
	ipResource := corev1.ResourceName(networkv1.NetworkResourceKeyPrefix + "my-network.IP")

	tests := []struct {
		name     string
		old      *corev1.Node
		new      *corev1.Node
		expected bool
	}{
		{
			name:     "no change",
			old:      ready,
			new:      ready.DeepCopy(),
			expected: false,
		},
		{
			name:     "kubelet heartbeat and unrelated status churn",
			old:      ready,
			new:      withHeartbeat(ready),
			expected: false,
		},
		{
			name:     "provider ID appears",
			old:      nodeWithProviderID(""),
			new:      ready,
			expected: true,
		},
		{
			name:     "unrelated annotation",
			old:      withAnnotations(ready, map[string]string{"example.com/unrelated": "a"}),
			new:      withAnnotations(ready, map[string]string{"example.com/unrelated": "b"}),
			expected: false,
		},
		{
			name:     "north interfaces annotation changes",
			old:      withAnnotations(ready, map[string]string{networkv1.NorthInterfacesAnnotationKey: "[]"}),
			new:      withAnnotations(ready, map[string]string{networkv1.NorthInterfacesAnnotationKey: `[{"network":"n1"}]`}),
			expected: true,
		},
		{
			name:     "multi-network annotation appears",
			old:      ready,
			new:      withAnnotations(ready, map[string]string{networkv1.MultiNetworkAnnotationKey: `[{"name":"n1"}]`}),
			expected: true,
		},
		{
			name:     "multi-network IP capacity changes",
			old:      withCapacity(ready, corev1.ResourceList{ipResource: resource.MustParse("8")}),
			new:      withCapacity(ready, corev1.ResourceList{ipResource: resource.MustParse("16")}),
			expected: true,
		},
		{
			name:     "unrelated capacity changes",
			old:      withCapacity(ready, corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("4")}),
			new:      withCapacity(ready, corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("8")}),
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := nodeNeedsStatusSync(tc.old, tc.new); got != tc.expected {
				t.Errorf("nodeNeedsStatusSync() = %v, want %v", got, tc.expected)
			}

			// The handler must agree with the predicate.
			f := newTestFixture(t)
			f.statusCtrl.handleNodeUpdate(tc.old, tc.new)
			gotEnqueued := f.statusCtrl.queue.Len() > 0
			if gotEnqueued != tc.expected {
				t.Errorf("handleNodeUpdate enqueued = %v, want %v", gotEnqueued, tc.expected)
			}
		})
	}
}

// TestStatusController_NodeUpdateFailsOpen checks that an unexpected object type
// is enqueued rather than silently dropped.
func TestStatusController_NodeUpdateFailsOpen(t *testing.T) {
	f := newTestFixture(t)
	f.statusCtrl.handleNodeUpdate("not-a-node", nodeWithProviderID(testProviderID))
	if f.statusCtrl.queue.Len() != 1 {
		t.Errorf("Expected an unexpected old-object type to fail open and enqueue, queue length is %d", f.statusCtrl.queue.Len())
	}
}

// TestStatusController_RunWaitsForNodeCache pins the cache-sync gate. Now that
// the controller resolves providerID through the Node lister, starting workers
// before that cache is populated would make every node look absent, which
// resolveNode reports as a not-ready node. Those are retried, so the damage is
// bounded, but it is a burst of pointless churn at every startup.
//
// The Node informer is deliberately never started here, so its cache can never
// sync and the gate must hold the key in the queue indefinitely. Without the
// gate a worker pops the key immediately; the NNC does not exist either, so the
// sync is a no-op that Forgets the key and the queue drains to zero.
func TestStatusController_RunWaitsForNodeCache(t *testing.T) {
	f := newTestFixture(t)
	stopCh := make(chan struct{})
	defer close(stopCh)

	f.statusCtrl.EnqueueNode(testNodeName)
	go f.statusCtrl.Run(1, stopCh)

	// A worker that is running at all would drain the queue in microseconds;
	// this is a scheduling margin, not a synchronization point.
	time.Sleep(250 * time.Millisecond)

	if got := f.statusCtrl.queue.Len(); got == 0 {
		t.Error("Expected Run to hold the key until the Node cache synced, but a worker processed it")
	}
}

// hasRange reports whether ranges contains exactly cidr.
func hasRange(ranges []string, cidr string) bool {
	for _, r := range ranges {
		if r == cidr {
			return true
		}
	}
	return false
}

// countSizeRequests returns the number of entries that are range sizes (e.g.
// "/28") rather than concrete CIDRs. A size entry is an addition: it asks GCE
// to allocate a new block.
func countSizeRequests(ranges []string) int {
	n := 0
	for _, r := range ranges {
		if r != "" && r[0] == '/' {
			n++
		}
	}
	return n
}

// TestUpdateAliasIPRanges_AddAndRemoveAreSeparateCalls covers a reconcile that
// both grows and shrinks a node's allocation.
//
// GCE rejects a single updateNetworkInterface that adds and removes alias IP
// ranges at once ("Cannot simultaneously add and remove alias IP ranges"), so
// the two have to be issued as separate requests. The instance's final state is
// identical either way, which is why this asserts on the requests themselves.
func TestUpdateAliasIPRanges_AddAndRemoveAreSeparateCalls(t *testing.T) {
	ctx := context.Background()
	f := newTestFixture(t)
	stopCh := make(chan struct{})
	defer close(stopCh)
	f.run(ctx, stopCh)

	cidrToRemove := "10.100.7.0/28"
	cidrToKeep := "10.100.8.0/28"

	instanceKey := meta.ZonalKey(testNodeName, testZone)
	instance := &compute.Instance{
		Name: testNodeName,
		Zone: testZone,
		NetworkInterfaces: []*compute.NetworkInterface{
			{
				Name:       "nic0",
				Network:    testNetworkURL,
				Subnetwork: "default",
				AliasIpRanges: []*compute.AliasIpRange{
					{IpCidrRange: cidrToRemove},
					{IpCidrRange: cidrToKeep},
				},
			},
		},
	}
	if err := f.fakeGCE.Compute().Instances().Insert(ctx, instanceKey, instance); err != nil {
		t.Fatalf("Failed to insert fake GCE instance: %v", err)
	}

	// Releasing one /28 while asking for 48 pods. The released range does not
	// count toward current capacity, so 16 usable IPs remain and two more /28
	// blocks are needed: this reconcile has both additions and removals.
	nnc := &nncv1.NodeNetworkConfig{
		ObjectMeta: metav1.ObjectMeta{Name: testNodeName},
		Spec: nncv1.NodeNetworkConfigSpec{
			Allocations: []nncv1.Allocation{
				{Network: "default", Pods: 48},
			},
			ReleasableCIDRs: []nncv1.PodCIDR{
				{Id: cidrToRemove, Network: "default", CIDR: cidrToRemove},
			},
		},
		Status: nncv1.NodeNetworkConfigStatus{
			PodCIDRs: []nncv1.PodCIDR{
				{Id: cidrToRemove, Network: "default", CIDR: cidrToRemove},
				{Id: cidrToKeep, Network: "default", CIDR: cidrToKeep},
			},
		},
	}
	if _, err := f.nncClient.NetworkingV1().NodeNetworkConfigs().Create(ctx, nnc, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create NodeNetworkConfig: %v", err)
	}
	f.informerFactory.WaitForCacheSync(stopCh)

	if err := f.reconcile(ctx, nnc, testProviderID); err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	calls := f.gceCalls.snapshot()
	if len(calls) != 2 {
		t.Fatalf("Expected 2 updateNetworkInterface calls (one remove, one add), got %d: %+v", len(calls), calls)
	}

	// First call: the removal, on its own. Dropping the released range and
	// keeping everything else, with nothing requested.
	remove := calls[0]
	if remove.ifaceName != "nic0" {
		t.Errorf("Expected first call on interface %q, got %q", "nic0", remove.ifaceName)
	}
	if hasRange(remove.ranges, cidrToRemove) {
		t.Errorf("Expected first call to drop released range %q, got %v", cidrToRemove, remove.ranges)
	}
	if !hasRange(remove.ranges, cidrToKeep) {
		t.Errorf("Expected first call to retain %q, got %v", cidrToKeep, remove.ranges)
	}
	if n := countSizeRequests(remove.ranges); n != 0 {
		t.Errorf("Expected first call to request no new blocks, got %d: %v", n, remove.ranges)
	}

	// Second call: the additions, on their own, against the post-removal list.
	add := calls[1]
	if add.ifaceName != "nic0" {
		t.Errorf("Expected second call on interface %q, got %q", "nic0", add.ifaceName)
	}
	if n := countSizeRequests(add.ranges); n != 2 {
		t.Errorf("Expected second call to request 2 new blocks, got %d: %v", n, add.ranges)
	}
	if hasRange(add.ranges, cidrToRemove) {
		t.Errorf("Expected second call to still exclude released range %q, got %v", cidrToRemove, add.ranges)
	}
	if !hasRange(add.ranges, cidrToKeep) {
		t.Errorf("Expected second call to retain %q, got %v", cidrToKeep, add.ranges)
	}

	// And the end state is what a single combined call would have produced.
	updatedInstance, err := f.fakeGCE.Compute().BetaInstances().Get(ctx, instanceKey)
	if err != nil {
		t.Fatalf("Failed to get updated GCE instance: %v", err)
	}
	iface := updatedInstance.NetworkInterfaces[0]
	if len(iface.AliasIpRanges) != 3 {
		t.Errorf("Expected 3 alias IP ranges, got %d: %v", len(iface.AliasIpRanges), iface.AliasIpRanges)
	}
	for _, r := range iface.AliasIpRanges {
		if r.IpCidrRange == cidrToRemove {
			t.Errorf("Released range %q is still attached: %v", cidrToRemove, iface.AliasIpRanges)
		}
	}
}

// TestUpdateAliasIPRanges_RemovalOnlyIsOneCall checks that the split does not
// cost an extra request when there is nothing to add.
func TestUpdateAliasIPRanges_RemovalOnlyIsOneCall(t *testing.T) {
	ctx := context.Background()
	f := newTestFixture(t)
	stopCh := make(chan struct{})
	defer close(stopCh)
	f.run(ctx, stopCh)

	cidrToRemove := "10.100.0.0/28"
	cidrToKeep := "10.100.1.0/28"

	instanceKey := meta.ZonalKey(testNodeName, testZone)
	instance := &compute.Instance{
		Name: testNodeName,
		Zone: testZone,
		NetworkInterfaces: []*compute.NetworkInterface{
			{
				Name:       "nic0",
				Network:    testNetworkURL,
				Subnetwork: "default",
				AliasIpRanges: []*compute.AliasIpRange{
					{IpCidrRange: cidrToRemove},
					{IpCidrRange: cidrToKeep},
				},
			},
		},
	}
	if err := f.fakeGCE.Compute().Instances().Insert(ctx, instanceKey, instance); err != nil {
		t.Fatalf("Failed to insert fake GCE instance: %v", err)
	}

	// 16 pods is satisfied by the range that is being kept, so nothing is added.
	nnc := &nncv1.NodeNetworkConfig{
		ObjectMeta: metav1.ObjectMeta{Name: testNodeName},
		Spec: nncv1.NodeNetworkConfigSpec{
			Allocations: []nncv1.Allocation{
				{Network: "default", Pods: 16},
			},
			ReleasableCIDRs: []nncv1.PodCIDR{
				{Id: cidrToRemove, Network: "default", CIDR: cidrToRemove},
			},
		},
		Status: nncv1.NodeNetworkConfigStatus{
			PodCIDRs: []nncv1.PodCIDR{
				{Id: cidrToRemove, Network: "default", CIDR: cidrToRemove},
				{Id: cidrToKeep, Network: "default", CIDR: cidrToKeep},
			},
		},
	}
	if _, err := f.nncClient.NetworkingV1().NodeNetworkConfigs().Create(ctx, nnc, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create NodeNetworkConfig: %v", err)
	}
	f.informerFactory.WaitForCacheSync(stopCh)

	if err := f.reconcile(ctx, nnc, testProviderID); err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	calls := f.gceCalls.snapshot()
	if len(calls) != 1 {
		t.Fatalf("Expected 1 updateNetworkInterface call, got %d: %+v", len(calls), calls)
	}
	if n := countSizeRequests(calls[0].ranges); n != 0 {
		t.Errorf("Expected no new blocks requested, got %d: %v", n, calls[0].ranges)
	}
	if hasRange(calls[0].ranges, cidrToRemove) {
		t.Errorf("Expected released range %q to be dropped, got %v", cidrToRemove, calls[0].ranges)
	}
}

// TestUpdateAliasIPRanges_AdditionOnlyIsOneCall checks the same for a pure
// scale-up, which is the common case.
func TestUpdateAliasIPRanges_AdditionOnlyIsOneCall(t *testing.T) {
	ctx := context.Background()
	f := newTestFixture(t)
	stopCh := make(chan struct{})
	defer close(stopCh)
	f.run(ctx, stopCh)

	existingCIDR := "10.100.5.0/28"

	instanceKey := meta.ZonalKey(testNodeName, testZone)
	instance := &compute.Instance{
		Name: testNodeName,
		Zone: testZone,
		NetworkInterfaces: []*compute.NetworkInterface{
			{
				Name:       "nic0",
				Network:    testNetworkURL,
				Subnetwork: "default",
				AliasIpRanges: []*compute.AliasIpRange{
					{IpCidrRange: existingCIDR},
				},
			},
		},
	}
	if err := f.fakeGCE.Compute().Instances().Insert(ctx, instanceKey, instance); err != nil {
		t.Fatalf("Failed to insert fake GCE instance: %v", err)
	}

	nnc := &nncv1.NodeNetworkConfig{
		ObjectMeta: metav1.ObjectMeta{Name: testNodeName},
		Spec: nncv1.NodeNetworkConfigSpec{
			Allocations: []nncv1.Allocation{
				{Network: "default", Pods: 32},
			},
		},
		Status: nncv1.NodeNetworkConfigStatus{
			PodCIDRs: []nncv1.PodCIDR{
				{Id: existingCIDR, Network: "default", CIDR: existingCIDR},
			},
		},
	}
	if _, err := f.nncClient.NetworkingV1().NodeNetworkConfigs().Create(ctx, nnc, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create NodeNetworkConfig: %v", err)
	}
	f.informerFactory.WaitForCacheSync(stopCh)

	if err := f.reconcile(ctx, nnc, testProviderID); err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	calls := f.gceCalls.snapshot()
	if len(calls) != 1 {
		t.Fatalf("Expected 1 updateNetworkInterface call, got %d: %+v", len(calls), calls)
	}
	if n := countSizeRequests(calls[0].ranges); n != 1 {
		t.Errorf("Expected 1 new block requested, got %d: %v", n, calls[0].ranges)
	}
	if !hasRange(calls[0].ranges, existingCIDR) {
		t.Errorf("Expected existing range %q to be retained, got %v", existingCIDR, calls[0].ranges)
	}
}

// TestUpdateAliasIPRanges_NoChangesIssuesNoCalls guards the other end: a node
// already at its target must not be written to at all.
func TestUpdateAliasIPRanges_NoChangesIssuesNoCalls(t *testing.T) {
	ctx := context.Background()
	f := newTestFixture(t)
	stopCh := make(chan struct{})
	defer close(stopCh)
	f.run(ctx, stopCh)

	existingCIDR := "10.100.5.0/28"

	instanceKey := meta.ZonalKey(testNodeName, testZone)
	instance := &compute.Instance{
		Name: testNodeName,
		Zone: testZone,
		NetworkInterfaces: []*compute.NetworkInterface{
			{
				Name:       "nic0",
				Network:    testNetworkURL,
				Subnetwork: "default",
				AliasIpRanges: []*compute.AliasIpRange{
					{IpCidrRange: existingCIDR},
				},
			},
		},
	}
	if err := f.fakeGCE.Compute().Instances().Insert(ctx, instanceKey, instance); err != nil {
		t.Fatalf("Failed to insert fake GCE instance: %v", err)
	}

	nnc := &nncv1.NodeNetworkConfig{
		ObjectMeta: metav1.ObjectMeta{Name: testNodeName},
		Spec: nncv1.NodeNetworkConfigSpec{
			Allocations: []nncv1.Allocation{
				{Network: "default", Pods: 16},
			},
		},
		Status: nncv1.NodeNetworkConfigStatus{
			PodCIDRs: []nncv1.PodCIDR{
				{Id: existingCIDR, Network: "default", CIDR: existingCIDR},
			},
		},
	}
	if _, err := f.nncClient.NetworkingV1().NodeNetworkConfigs().Create(ctx, nnc, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Failed to create NodeNetworkConfig: %v", err)
	}
	f.informerFactory.WaitForCacheSync(stopCh)

	if err := f.reconcile(ctx, nnc, testProviderID); err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	if calls := f.gceCalls.snapshot(); len(calls) != 0 {
		t.Errorf("Expected no updateNetworkInterface calls, got %d: %+v", len(calls), calls)
	}
}
