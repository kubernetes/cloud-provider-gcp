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
	"testing"
	"time"

	nncv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/nodenetworkconfig/v1"
	nncfake "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/clientset/versioned/fake"
	nncinformers "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/informers/externalversions"
	gcloud "github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud"
	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/meta"
	compute "google.golang.org/api/compute/v1"
	computebeta "google.golang.org/api/compute/v0.beta"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	clientgotesting "k8s.io/client-go/testing"
	clocktesting "k8s.io/utils/clock/testing"
	gce "k8s.io/cloud-provider-gcp/providers/gce"
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
	t               *testing.T
	kubeClient      *k8sfake.Clientset
	nncClient       *nncfake.Clientset
	informerFactory nncinformers.SharedInformerFactory
	fakeGCE         *gce.Cloud
	specCtrl        *NodeNetworkConfigSpecController
	statusCtrl      *NodeNetworkConfigStatusController
	fakeClock       *clocktesting.FakeClock
}

func newTestFixture(t *testing.T) *testFixture {
	kubeClient := k8sfake.NewSimpleClientset()
	nncClient := nncfake.NewSimpleClientset()
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
	mockInstances.UpdateNetworkInterfaceHook = updateNetworkInterfaceHook

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
		nodeInformer.Lister(),
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
		t:               t,
		kubeClient:      kubeClient,
		nncClient:       nncClient,
		informerFactory: informerFactory,
		fakeGCE:         fakeGCE,
		specCtrl:        specCtrl,
		statusCtrl:      statusCtrl,
		fakeClock:       fakeClock,
	}
}

func (f *testFixture) run(ctx context.Context, stopCh <-chan struct{}) {
	f.informerFactory.Start(stopCh)
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

	// 1. Create a fake GCE instance in the fake cloud
	instanceKey := meta.ZonalKey(testNodeName, testZone)
	instance := &compute.Instance{
		Name: testNodeName,
		Zone: testZone,
		NetworkInterfaces: []*compute.NetworkInterface{
			{
				Name:  "nic0",
				Network: testNetworkURL,
				Subnetwork: "default",
			},
		},
	}
	// Insert GA instance (fake GCE shares DB between GA and Beta)
	err := f.fakeGCE.Compute().Instances().Insert(ctx, instanceKey, instance)
	if err != nil {
		t.Fatalf("Failed to insert fake GCE instance: %v", err)
	}

	// 2. Create a NodeNetworkConfig with a request for 48 pods
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

	// 3. Run reconcile
	err = f.reconcile(ctx, nnc, testProviderID)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	// 4. Verify GCE Instance has new alias IPs
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

	// 5. Verify NodeNetworkConfig Status is updated
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

	// 1. Create a fake GCE instance with 2 alias IPs
	instanceKey := meta.ZonalKey(testNodeName, testZone)
	instance := &compute.Instance{
		Name: testNodeName,
		Zone: testZone,
		NetworkInterfaces: []*compute.NetworkInterface{
			{
				Name:  "nic0",
				Network: testNetworkURL,
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

	// 2. Create NodeNetworkConfig with:
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

	// 3. Run reconcile
	err = f.reconcile(ctx, nnc, testProviderID)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	// 4. Verify GCE Instance has only the kept alias IP
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

	// 5. Verify NodeNetworkConfig Status is updated (only kept CIDR remains)
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

	// 1. Create GCE instance with 1 alias IP
	instanceKey := meta.ZonalKey(testNodeName, testZone)
	instance := &compute.Instance{
		Name: testNodeName,
		Zone: testZone,
		NetworkInterfaces: []*compute.NetworkInterface{
			{
				Name:  "nic0",
				Network: testNetworkURL,
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

	// 2. Create NodeNetworkConfig where Spec matches Status capacity (16 desired, 16 actual)
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
	
	// 3. Run reconcile
	err = f.reconcile(ctx, nnc, testProviderID)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	// 4. Verify GCE Instance remains unchanged
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

	// 1. Create NodeNetworkConfig with a request
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

	// 2. Run reconcile. It should fail because the instance doesn't exist in GCE.
	err = f.reconcile(ctx, nnc, testProviderID)
	if err == nil {
		t.Fatal("Expected reconcile to fail, but it succeeded")
	}

	// 3. Verify NodeNetworkConfig Status has False Ready condition with correct reason
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
			// Resolve the alias IPs (allocate for '/28' sizes)
			resolvedRanges := resolveMockAliasIPs(iface.AliasIpRanges)
			instance.NetworkInterfaces[i].AliasIpRanges = resolvedRanges
			instance.NetworkInterfaces[i].Fingerprint = fmt.Sprintf("new-fingerprint-%d", len(resolvedRanges)) // dummy fingerprint change
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

	// 1. Create a fake GCE instance with 0 alias IPs (clean state)
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

	// 2. Part 1: Create NNC with an invalid CIDR in Status.
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

	// 3. Part 2: IPv6 CIDR in Status.
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

	// 1. Create a fake GCE instance in the fake cloud
	instanceKey := meta.ZonalKey(testNodeName, testZone)
	instance := &compute.Instance{
		Name: testNodeName,
		Zone: testZone,
		NetworkInterfaces: []*compute.NetworkInterface{
			{
				Name:    "nic0",
				Network: testNetworkURL,
				Subnetwork: "default",
			},
		},
	}
	err := f.fakeGCE.Compute().Instances().Insert(ctx, instanceKey, instance)
	if err != nil {
		t.Fatalf("Failed to insert fake GCE instance: %v", err)
	}

	// 2. Create a NodeNetworkConfig with a request for 32 pods (needs 2 blocks of /28)
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

	// 3. First reconcile run. It should fail on the status update.
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

	// 4. Second reconcile run (Simulated Retry).
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

	// 1. Create a GCE instance with TWO network interfaces
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

	// 2. Create NodeNetworkConfig requesting allocations on BOTH networks
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

	// 3. Run reconcile
	err = f.reconcile(ctx, nnc, testProviderID)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	// 4. Verify GCE Instance has alias IPs on BOTH interfaces
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

	// 1. Create a fake GCE instance
	instanceKey := meta.ZonalKey(testNodeName, testZone)
	instance := &compute.Instance{
		Name: testNodeName,
		Zone: testZone,
		NetworkInterfaces: []*compute.NetworkInterface{
			{
				Name:    "nic0",
				Network: testNetworkURL,
				Subnetwork: "default",
			},
		},
	}
	err := f.fakeGCE.Compute().Instances().Insert(ctx, instanceKey, instance)
	if err != nil {
		t.Fatalf("Failed to insert fake GCE instance: %v", err)
	}

	// 2. Create NNC requesting 32 pods (needs 2 blocks of /28)
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

	// 3. First run: allocates 2 blocks in GCE, fails status write.
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

	// 4. Manually mutate GCE behind the back of the cache (add a 3rd block!)
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

	// 5. Scenario A: Retry immediately (Fresh Cache, age = 0s < 10s TTL)
	// The simulated reactor only fails on the 2nd update, so this retry will naturally succeed.

	unsyncedNNC, err := f.nncClient.NetworkingV1().NodeNetworkConfigs().Get(ctx, testNodeName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get NNC: %v", err)
	}

	err = f.reconcile(ctx, unsyncedNNC, testProviderID)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	// ASSERTION: Status should ONLY contain the 2 blocks from the fresh cache!
	// It should NOT contain the 3rd block we added directly to GCE, because it hit the cache and skipped GCE GET!
	finalNNC, err := f.nncClient.NetworkingV1().NodeNetworkConfigs().Get(ctx, testNodeName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get NNC: %v", err)
	}
	if len(finalNNC.Status.PodCIDRs) != 2 {
		t.Errorf("Expected 2 PodCIDRs in status (cache hit), got %d: %v", len(finalNNC.Status.PodCIDRs), finalNNC.Status.PodCIDRs)
	}

	// 6. Scenario B: Expire the Cache & Sync again
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

	// 1. Create a GCE instance with existing alias IPs
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

	// 2. Create NNC with empty Status
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

	// 3. Directly invoke status controller syncNode (simulating workqueue execution)
	err = f.statusCtrl.syncNode(testNodeName)
	if err != nil {
		t.Fatalf("Status sync failed: %v", err)
	}

	// 4. Verify NNC Status is populated
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

