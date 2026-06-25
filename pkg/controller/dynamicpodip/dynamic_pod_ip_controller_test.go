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

	nncv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/nodenetworkconfig/v1"
	nncfake "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/clientset/versioned/fake"
	nncinformers "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/informers/externalversions"
	gcloud "github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud"
	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/meta"
	compute "google.golang.org/api/compute/v1"
	computebeta "google.golang.org/api/compute/v0.beta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	gce "k8s.io/cloud-provider-gcp/providers/gce"
)

const (
	testNodeName = "test-node"
	testZone     = "us-central1-a"
	testProject  = "test-project"
)

type testFixture struct {
	t               *testing.T
	kubeClient      *k8sfake.Clientset
	nncClient       *nncfake.Clientset
	informerFactory nncinformers.SharedInformerFactory
	fakeGCE         *gce.Cloud
	controller      *Controller
}

func newTestFixture(t *testing.T) *testFixture {
	kubeClient := k8sfake.NewSimpleClientset()
	nncClient := nncfake.NewSimpleClientset()
	informerFactory := nncinformers.NewSharedInformerFactory(nncClient, 0)
	nncInformer := informerFactory.Networking().V1().NodeNetworkConfigs()

	testClusterValues := gce.DefaultTestClusterValues()
	testClusterValues.ProjectID = testProject
	testClusterValues.ZoneName = testZone
	fakeGCE := gce.NewFakeGCECloud(testClusterValues)

	// Register the UpdateNetworkInterface hook to simulate GCE mutation and allocation
	mockInstances, ok := fakeGCE.Compute().BetaInstances().(*gcloud.MockBetaInstances)
	if !ok {
		t.Fatalf("Failed to cast BetaInstances to MockBetaInstances")
	}
	mockInstances.UpdateNetworkInterfaceHook = updateNetworkInterfaceHook

	controller := NewController(
		kubeClient,
		nncClient,
		nncInformer,
		fakeGCE,
	)

	return &testFixture{
		t:               t,
		kubeClient:      kubeClient,
		nncClient:       nncClient,
		informerFactory: informerFactory,
		fakeGCE:         fakeGCE,
		controller:      controller,
	}
}

func (f *testFixture) run(ctx context.Context, stopCh <-chan struct{}) {
	f.informerFactory.Start(stopCh)
	// We don't call controller.Run because it blocks. We will call reconcile directly in tests
	// to have fine-grained control and synchronous execution.
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
				Network: "default",
				Subnetwork: "default",
			},
		},
	}
	// Insert GA instance (fake GCE shares DB between GA and Beta)
	err := f.fakeGCE.Compute().Instances().Insert(ctx, instanceKey, instance)
	if err != nil {
		t.Fatalf("Failed to insert fake GCE instance: %v", err)
	}

	// 2. Create a NodeNetworkConfig with a request for 32 pods
	nnc := &nncv1.NodeNetworkConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNodeName,
		},
		Spec: nncv1.NodeNetworkConfigSpec{
			Allocations: []nncv1.Allocation{
				{
					Network: "default",
					Pods:    32, // Requests 32 pods -> should result in 2x /28 blocks (32 IPs)
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
	err = f.controller.reconcile(ctx, nnc)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	// 4. Verify GCE Instance has new alias IPs
	updatedInstance, err := f.fakeGCE.Compute().BetaInstances().Get(ctx, instanceKey)
	if err != nil {
		t.Fatalf("Failed to get updated GCE instance: %v", err)
	}

	iface := updatedInstance.NetworkInterfaces[0]
	// We expect 2 alias IP ranges added (each of size /28, GCE fake auto-allocates IPs)
	if len(iface.AliasIpRanges) != 2 {
		t.Errorf("Expected 2 alias IP ranges, got %d: %v", len(iface.AliasIpRanges), iface.AliasIpRanges)
	}
	for _, r := range iface.AliasIpRanges {
		// GCE fake assigns IPs like "10.0.0.0/28" etc. We just check if it is not empty.
		if r.IpCidrRange == "" {
			t.Error("Expected non-empty IpCidrRange")
		}
	}

	// 5. Verify NodeNetworkConfig Status is updated
	updatedNNC, err := f.nncClient.NetworkingV1().NodeNetworkConfigs().Get(ctx, testNodeName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get updated NodeNetworkConfig: %v", err)
	}

	if len(updatedNNC.Status.PodCIDRs) != 2 {
		t.Errorf("Expected 2 PodCIDRs in status, got %d: %v", len(updatedNNC.Status.PodCIDRs), updatedNNC.Status.PodCIDRs)
	}
	for _, pc := range updatedNNC.Status.PodCIDRs {
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
				Network: "default",
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
	err = f.controller.reconcile(ctx, nnc)
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
				Network: "default",
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
	err = f.controller.reconcile(ctx, nnc)
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
					Pods:    16,
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
	err = f.controller.reconcile(ctx, nnc)
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
