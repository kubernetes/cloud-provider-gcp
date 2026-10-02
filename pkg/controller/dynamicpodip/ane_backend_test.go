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
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	nncv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/nodenetworkconfig/v1"
	nncfake "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/clientset/versioned/fake"
	nncinformers "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/informers/externalversions"
	gcloud "github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud"
	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/meta"
	computebeta "google.golang.org/api/compute/v0.beta"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/cloud-provider-gcp/pkg/controller/dynamicpodip/ane"
	gce "k8s.io/cloud-provider-gcp/providers/gce"
	"k8s.io/utils/clock"
)

func TestANEModeDetection(t *testing.T) {
	aliasRanges := NewAliasRangesBackend(nil)
	if aliasRanges.IsANEMode() {
		t.Fatalf("expected aliasRangesBackend.IsANEMode() == false")
	}

	aneClient := ane.NewFakeClient()
	aneBackend := NewANEBackend(nil, aneClient)
	if !aneBackend.IsANEMode() {
		t.Fatalf("expected aneBackend.IsANEMode() == true")
	}
}

func TestANEBackendGetNetworkInterfaces(t *testing.T) {
	ctx := context.Background()
	testClusterValues := gce.DefaultTestClusterValues()
	testClusterValues.ProjectID = testProject
	testClusterValues.ZoneName = testZone
	testClusterValues.NetworkURL = testNetworkURL
	fakeGCE := gce.NewFakeGCECloud(testClusterValues)

	fakeANE := ane.NewFakeClient()
	backend := NewANEBackend(fakeGCE, fakeANE)

	instanceURL := fmt.Sprintf("projects/%s/zones/%s/instances/%s", testProject, testZone, testNodeName)

	// Add an active ANE
	_, err := fakeANE.Insert(ctx, testProject, testZone, &ane.AliasNetworkEndpoint{
		Name:       "ane-active-1",
		Subnetwork: fakeGCE.SubnetworkURL(),
		Host:       &ane.Host{Instance: instanceURL},
		Status:     &ane.Status{State: ane.StateActive},
		Aliases: map[string]*ane.Alias{
			ane.DefaultAliasName: {
				EffectiveIPAddress: "10.128.0.10",
			},
		},
	})
	if err != nil {
		t.Fatalf("Insert active ANE: %v", err)
	}

	// Add a creating/inactive ANE (should be skipped)
	_, err = fakeANE.Insert(ctx, testProject, testZone, &ane.AliasNetworkEndpoint{
		Name:       "ane-creating-2",
		Subnetwork: fakeGCE.SubnetworkURL(),
		Host:       &ane.Host{Instance: instanceURL},
		Status:     &ane.Status{State: ane.StateCreating},
		Aliases: map[string]*ane.Alias{
			ane.DefaultAliasName: {
				EffectiveIPAddress: "10.128.0.20",
			},
		},
	})
	if err != nil {
		t.Fatalf("Insert inactive ANE: %v", err)
	}

	ifaces, err := backend.GetNetworkInterfaces(ctx, testProviderID)
	if err != nil {
		t.Fatalf("GetNetworkInterfaces: %v", err)
	}

	if len(ifaces) != 1 {
		t.Fatalf("expected 1 interface, got %d", len(ifaces))
	}
	if len(ifaces[0].AliasIPRanges) != 1 {
		t.Fatalf("expected 1 alias IP range, got %d: %v", len(ifaces[0].AliasIPRanges), ifaces[0].AliasIPRanges)
	}
	expectedCIDR := "10.128.0.10/32"
	if ifaces[0].AliasIPRanges[0] != expectedCIDR {
		t.Fatalf("expected %q, got %q", expectedCIDR, ifaces[0].AliasIPRanges[0])
	}
}

func TestANEBackendMutateAdditionsAndRemovals(t *testing.T) {
	ctx := context.Background()
	testClusterValues := gce.DefaultTestClusterValues()
	testClusterValues.ProjectID = testProject
	testClusterValues.ZoneName = testZone
	testClusterValues.NetworkURL = testNetworkURL
	fakeGCE := gce.NewFakeGCECloud(testClusterValues)

	fakeANE := ane.NewFakeClient()
	backend := NewANEBackend(fakeGCE, fakeANE)

	// Mutate: Add 2 ANEs
	err := backend.Mutate(ctx, testProviderID, testNetworkURL, []string{"/32", "/32"}, nil, nil)
	if err != nil {
		t.Fatalf("Mutate additions: %v", err)
	}

	if len(fakeANE.Endpoints) != 2 {
		t.Fatalf("expected 2 endpoints in fakeANE, got %d", len(fakeANE.Endpoints))
	}

	ifaces, err := backend.GetNetworkInterfaces(ctx, testProviderID)
	if err != nil {
		t.Fatalf("GetNetworkInterfaces: %v", err)
	}
	if len(ifaces[0].AliasIPRanges) != 2 {
		t.Fatalf("expected 2 alias ranges, got %d", len(ifaces[0].AliasIPRanges))
	}

	// Mutate: Remove 1 ANE
	toRemove := ifaces[0].AliasIPRanges[0]
	err = backend.Mutate(ctx, testProviderID, testNetworkURL, nil, []string{toRemove}, nil)
	if err != nil {
		t.Fatalf("Mutate removals: %v", err)
	}

	ifacesAfter, err := backend.GetNetworkInterfaces(ctx, testProviderID)
	if err != nil {
		t.Fatalf("GetNetworkInterfaces after removal: %v", err)
	}
	if len(ifacesAfter[0].AliasIPRanges) != 1 {
		t.Fatalf("expected 1 alias range remaining, got %d", len(ifacesAfter[0].AliasIPRanges))
	}
	if ifacesAfter[0].AliasIPRanges[0] == toRemove {
		t.Fatalf("expected CIDR %q to be removed", toRemove)
	}
}

func TestANEControllerReconcileFlow(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	kubeClient := k8sfake.NewSimpleClientset()
	nncClient := nncfake.NewSimpleClientset()

	nodeInformerFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	nodeInformer := nodeInformerFactory.Core().V1().Nodes()

	nncInformerFactory := nncinformers.NewSharedInformerFactory(nncClient, 0)
	nncInformer := nncInformerFactory.Networking().V1().NodeNetworkConfigs()

	testClusterValues := gce.DefaultTestClusterValues()
	testClusterValues.ProjectID = testProject
	testClusterValues.ZoneName = testZone
	testClusterValues.NetworkURL = testNetworkURL
	fakeGCE := gce.NewFakeGCECloud(testClusterValues)
	fakeANE := ane.NewFakeClient()
	backend := NewANEBackend(fakeGCE, fakeANE)

	loader := func(ctx context.Context, providerID string) ([]*networkInterface, error) {
		return backend.GetNetworkInterfaces(ctx, providerID)
	}
	gceCache := NewGCECache(loader, 10*time.Second, clock.RealClock{})

	statusCtrl := NewStatusController(
		kubeClient,
		nncClient,
		nncInformer.Lister(),
		nodeInformer,
		fakeGCE,
		gceCache,
		clock.RealClock{},
	)

	specCtrl := NewSpecControllerWithBackend(
		kubeClient,
		nncClient,
		nncInformer,
		nodeInformer,
		fakeGCE,
		gceCache,
		statusCtrl,
		nil,
		backend,
	)

	// Create Node
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: testNodeName},
		Spec: corev1.NodeSpec{
			ProviderID: testProviderID,
		},
	}
	if _, err := kubeClient.CoreV1().Nodes().Create(ctx, node, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Create Node: %v", err)
	}
	if err := nodeInformer.Informer().GetStore().Add(node); err != nil {
		t.Fatalf("Add Node to cache: %v", err)
	}

	// Create NNC requesting 2 Pod IPs
	nnc := &nncv1.NodeNetworkConfig{
		ObjectMeta: metav1.ObjectMeta{Name: testNodeName},
		Spec: nncv1.NodeNetworkConfigSpec{
			Allocations: []nncv1.Allocation{
				{Network: "default", Pods: 2},
			},
		},
	}
	if _, err := nncClient.NetworkingV1().NodeNetworkConfigs().Create(ctx, nnc, metav1.CreateOptions{}); err != nil {
		t.Fatalf("Create NNC: %v", err)
	}
	if err := nncInformer.Informer().GetStore().Add(nnc); err != nil {
		t.Fatalf("Add NNC to cache: %v", err)
	}

	// Run spec controller sync
	if err := specCtrl.syncNode(testNodeName); err != nil {
		t.Fatalf("SpecCtrl.syncNode: %v", err)
	}

	// Verify 2 ANEs were created
	if len(fakeANE.Endpoints) != 2 {
		t.Fatalf("expected 2 ANEs created in fakeANE, got %d", len(fakeANE.Endpoints))
	}

	// Run status controller sync
	if err := statusCtrl.syncNode(testNodeName); err != nil {
		t.Fatalf("StatusCtrl.syncNode: %v", err)
	}

	// Check NNC Status
	updatedNNC, err := nncClient.NetworkingV1().NodeNetworkConfigs().Get(ctx, testNodeName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get NNC: %v", err)
	}
	if len(updatedNNC.Status.PodCIDRs) != 2 {
		t.Fatalf("expected 2 PodCIDRs in status, got %d: %v", len(updatedNNC.Status.PodCIDRs), updatedNNC.Status.PodCIDRs)
	}

	for _, cidr := range updatedNNC.Status.PodCIDRs {
		if cidr.Condition == nil || cidr.Condition.Status != metav1.ConditionTrue {
			t.Errorf("expected PodCIDR %q condition to be True, got %v", cidr.CIDR, cidr.Condition)
		}
	}

	// Now release 1 Pod IP: Pods requested goes down to 1, and 1 CIDR is put in ReleasableCIDRs
	releasableCIDR := updatedNNC.Status.PodCIDRs[0].CIDR
	updatedNNC.Spec.Allocations[0].Pods = 1
	updatedNNC.Spec.ReleasableCIDRs = []nncv1.PodCIDR{
		{Network: "default", CIDR: releasableCIDR},
	}
	if _, err := nncClient.NetworkingV1().NodeNetworkConfigs().Update(ctx, updatedNNC, metav1.UpdateOptions{}); err != nil {
		t.Fatalf("Update NNC with ReleasableCIDRs: %v", err)
	}
	if err := nncInformer.Informer().GetStore().Update(updatedNNC); err != nil {
		t.Fatalf("Update NNC in cache: %v", err)
	}

	// Sync spec controller
	if err := specCtrl.syncNode(testNodeName); err != nil {
		t.Fatalf("SpecCtrl.syncNode release: %v", err)
	}

	// Verify 1 ANE remains in fakeANE
	if len(fakeANE.Endpoints) != 1 {
		t.Fatalf("expected 1 ANE remaining in fakeANE, got %d", len(fakeANE.Endpoints))
	}

	// Sync status controller
	if err := statusCtrl.syncNode(testNodeName); err != nil {
		t.Fatalf("StatusCtrl.syncNode release: %v", err)
	}

	// Check NNC Status has 1 CIDR left
	finalNNC, err := nncClient.NetworkingV1().NodeNetworkConfigs().Get(ctx, testNodeName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get final NNC: %v", err)
	}
	if len(finalNNC.Status.PodCIDRs) != 1 {
		t.Fatalf("expected 1 PodCIDR in final status, got %d: %v", len(finalNNC.Status.PodCIDRs), finalNNC.Status.PodCIDRs)
	}
	if finalNNC.Status.PodCIDRs[0].CIDR == releasableCIDR {
		t.Fatalf("expected released CIDR %q to be removed from status", releasableCIDR)
	}
}

func TestStartControllersANEActivation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	kubeClient := k8sfake.NewSimpleClientset()
	nncClient := nncfake.NewSimpleClientset()
	nodeInformerFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	nodeInformer := nodeInformerFactory.Core().V1().Nodes()

	testClusterValues := gce.DefaultTestClusterValues()
	fakeGCE := gce.NewFakeGCECloud(testClusterValues)

	trigger, ctrl, started, err := StartControllers(
		ctx,
		Options{
			EnableDynamicPodIPController: true,
			UseAliasNetworkEndpoints:    true,
		},
		kubeClient,
		nncClient,
		nodeInformer,
		fakeGCE,
	)
	if err != nil {
		t.Fatalf("StartControllers failed: %v", err)
	}
	if !started {
		t.Fatalf("expected started == true")
	}
	if trigger == nil {
		t.Fatalf("expected non-nil trigger")
	}
	if ctrl == nil {
		t.Fatalf("expected non-nil ctrl")
	}
}

func TestCalculateAdditions(t *testing.T) {
	aliasRanges := NewAliasRangesBackend(nil)
	aneBackend := NewANEBackend(nil, nil)

	// Test aliasRanges backend: 1 pod should request 1 block of /28
	adds1 := aliasRanges.CalculateAdditions("default", 1, nil)
	if len(adds1) != 1 || adds1[0] != DefaultBlockSize {
		t.Errorf("expected 1 block of %s, got %v", DefaultBlockSize, adds1)
	}

	// 17 pods should request 2 blocks of /28
	adds17 := aliasRanges.CalculateAdditions("default", 17, nil)
	if len(adds17) != 2 || adds17[0] != DefaultBlockSize || adds17[1] != DefaultBlockSize {
		t.Errorf("expected 2 blocks of %s, got %v", DefaultBlockSize, adds17)
	}

	// Test aneBackend: 5 pods should request 5 individual /32 allocations
	addsANE := aneBackend.CalculateAdditions("default", 5, nil)
	if len(addsANE) != 5 {
		t.Fatalf("expected 5 additions, got %d", len(addsANE))
	}
	for i, add := range addsANE {
		if add != "/32" {
			t.Errorf("addition[%d] = %q, want /32", i, add)
		}
	}
}

func TestGenerateANEName_RFC1035Compliance(t *testing.T) {
	rfc1035Regex := regexp.MustCompile(`^[a-z]([-a-z0-9]*[a-z0-9])?$`)

	testNodeNames := []string{
		"node-1",
		"gke-cluster-pool-1-abc1234-def5",
		"a-very-long-node-name-that-is-forty-five-characters",
		"gke-my-massive-production-cluster-name-default-pool-abcdef1234567890-node-123",
		"node.with.dots.and--dashes--",
	}

	for _, nodeName := range testNodeNames {
		for i := 0; i < 5; i++ {
			name := generateANEName(nodeName, i)
			if len(name) > 63 {
				t.Errorf("name %q exceeds 63 characters (length: %d) for node %q", name, len(name), nodeName)
			}
			if !rfc1035Regex.MatchString(name) {
				t.Errorf("name %q does not match RFC 1035 regex for node %q", name, nodeName)
			}
		}
	}
}

func TestANEBackendMutate_IdempotentDelete(t *testing.T) {
	ctx := context.Background()
	testClusterValues := gce.DefaultTestClusterValues()
	testClusterValues.ProjectID = testProject
	testClusterValues.ZoneName = testZone
	testClusterValues.NetworkURL = testNetworkURL
	fakeGCE := gce.NewFakeGCECloud(testClusterValues)

	fakeANE := ane.NewFakeClient()
	backend := NewANEBackend(fakeGCE, fakeANE)

	// Removing an IP that has no corresponding ANE in GCE should be a silent no-op (no error)
	err := backend.Mutate(ctx, testProviderID, testNetworkURL, nil, []string{"10.128.0.99/32"}, nil)
	if err != nil {
		t.Fatalf("expected idempotent removal to succeed, got: %v", err)
	}
}

func TestANEBackendMutate_ConcurrentAdditionsAndRemovals(t *testing.T) {
	ctx := context.Background()
	testClusterValues := gce.DefaultTestClusterValues()
	testClusterValues.ProjectID = testProject
	testClusterValues.ZoneName = testZone
	testClusterValues.NetworkURL = testNetworkURL
	fakeGCE := gce.NewFakeGCECloud(testClusterValues)

	fakeANE := ane.NewFakeClient()
	backend := NewANEBackend(fakeGCE, fakeANE)

	// Test concurrent addition of 16 ANEs (two full batches under concurrency=8)
	var additions []string
	for i := 0; i < 16; i++ {
		additions = append(additions, "/32")
	}

	err := backend.Mutate(ctx, testProviderID, testNetworkURL, additions, nil, nil)
	if err != nil {
		t.Fatalf("Mutate concurrent additions failed: %v", err)
	}

	ifaces, err := backend.GetNetworkInterfaces(ctx, testProviderID)
	if err != nil {
		t.Fatalf("GetNetworkInterfaces failed: %v", err)
	}
	if len(ifaces) != 1 || len(ifaces[0].AliasIPRanges) != 16 {
		t.Fatalf("expected 16 alias IP ranges, got %d", len(ifaces[0].AliasIPRanges))
	}

	// Verify all 16 IP CIDRs are unique
	ipSet := make(map[string]bool)
	for _, cidr := range ifaces[0].AliasIPRanges {
		if ipSet[cidr] {
			t.Errorf("duplicate IP CIDR found: %q", cidr)
		}
		ipSet[cidr] = true
	}

	// Test concurrent removal of 8 ANEs
	var toRemove []string
	for i := 0; i < 8; i++ {
		toRemove = append(toRemove, ifaces[0].AliasIPRanges[i])
	}

	err = backend.Mutate(ctx, testProviderID, testNetworkURL, nil, toRemove, nil)
	if err != nil {
		t.Fatalf("Mutate concurrent removals failed: %v", err)
	}

	ifacesAfter, err := backend.GetNetworkInterfaces(ctx, testProviderID)
	if err != nil {
		t.Fatalf("GetNetworkInterfaces after removal failed: %v", err)
	}
	if len(ifacesAfter[0].AliasIPRanges) != 8 {
		t.Fatalf("expected 8 alias IP ranges remaining, got %d", len(ifacesAfter[0].AliasIPRanges))
	}

	// Verify the removed CIDRs are actually gone
	for _, rem := range toRemove {
		for _, remaining := range ifacesAfter[0].AliasIPRanges {
			if remaining == rem {
				t.Errorf("removed CIDR %q still found in interfaces", rem)
			}
		}
	}

	// Test context cancellation during mutation
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately
	err = backend.Mutate(canceledCtx, testProviderID, testNetworkURL, []string{"/32"}, nil, nil)
	if err == nil {
		t.Fatal("expected error on canceled context, got nil")
	}
}

func TestANEBackendMutate_MultiInterfaceSubnetTargeting(t *testing.T) {
	ctx := context.Background()
	testClusterValues := gce.DefaultTestClusterValues()
	testClusterValues.ProjectID = testProject
	testClusterValues.ZoneName = testZone
	testClusterValues.NetworkURL = testNetworkURL
	fakeGCE := gce.NewFakeGCECloud(testClusterValues)

	netA := fmt.Sprintf("https://www.googleapis.com/compute/v1/projects/%s/global/networks/net-a", testProject)
	subnetA := fmt.Sprintf("https://www.googleapis.com/compute/v1/projects/%s/regions/%s/subnetworks/subnet-a", testProject, "us-central1")
	netB := fmt.Sprintf("https://www.googleapis.com/compute/v1/projects/%s/global/networks/net-b", testProject)
	subnetB := fmt.Sprintf("https://www.googleapis.com/compute/v1/projects/%s/regions/%s/subnetworks/subnet-b", testProject, "us-central1")

	instance := &computebeta.Instance{
		Name: testNodeName,
		Zone: testZone,
		NetworkInterfaces: []*computebeta.NetworkInterface{
			{
				Name:       "nic0",
				Network:    netA,
				Subnetwork: subnetA,
			},
			{
				Name:       "nic1",
				Network:    netB,
				Subnetwork: subnetB,
			},
		},
	}
	mockInstances, ok := fakeGCE.Compute().BetaInstances().(*gcloud.MockBetaInstances)
	if !ok {
		t.Fatalf("Failed to cast BetaInstances to MockBetaInstances")
	}
	instanceKey := meta.ZonalKey(testNodeName, testZone)
	mockInstances.Objects[*instanceKey] = &gcloud.MockInstancesObj{Obj: instance}

	fakeANE := ane.NewFakeClient()
	backend := NewANEBackend(fakeGCE, fakeANE)

	// Mutate targeting netB -> should resolve subnetB for the ANE
	err := backend.Mutate(ctx, testProviderID, netB, []string{"/32"}, nil, nil)
	if err != nil {
		t.Fatalf("Mutate netB failed: %v", err)
	}

	var foundSubnetB bool
	for _, ep := range fakeANE.Endpoints {
		if ep.Subnetwork == subnetB {
			foundSubnetB = true
			break
		}
	}
	if !foundSubnetB {
		t.Fatalf("expected ANE created with subnetwork %q, got: %v", subnetB, fakeANE.Endpoints)
	}

	// Mutate targeting netA -> should resolve subnetA for the ANE
	err = backend.Mutate(ctx, testProviderID, netA, []string{"/32"}, nil, nil)
	if err != nil {
		t.Fatalf("Mutate netA failed: %v", err)
	}

	var foundSubnetA bool
	for _, ep := range fakeANE.Endpoints {
		if ep.Subnetwork == subnetA {
			foundSubnetA = true
			break
		}
	}
	if !foundSubnetA {
		t.Fatalf("expected ANE created with subnetwork %q, got: %v", subnetA, fakeANE.Endpoints)
	}

	// GetNetworkInterfaces should return both nic0 and nic1 with their respective ANE IPs
	ifaces, err := backend.GetNetworkInterfaces(ctx, testProviderID)
	if err != nil {
		t.Fatalf("GetNetworkInterfaces failed: %v", err)
	}
	if len(ifaces) != 2 {
		t.Fatalf("expected 2 interfaces, got %d", len(ifaces))
	}
	if len(ifaces[0].AliasIPRanges) != 1 || len(ifaces[1].AliasIPRanges) != 1 {
		t.Fatalf("expected 1 IP on nic0 and 1 IP on nic1, got nic0=%v, nic1=%v", ifaces[0].AliasIPRanges, ifaces[1].AliasIPRanges)
	}
}

func TestANEBackend_FailFastOnSilentScrubbing(t *testing.T) {
	ctx := context.Background()
	fakeANE := ane.NewFakeClient()
	fakeGCE := gce.NewFakeGCECloud(gce.DefaultTestClusterValues())
	backend := NewANEBackend(fakeGCE, fakeANE)

	testProviderID := "gce://test-project/us-central1-b/test-node"

	// Insert an active endpoint that has missing aliases (simulating silent scrubbing)
	fakeANE.Endpoints["scrubbed-ane"] = &ane.AliasNetworkEndpoint{
		Name: "scrubbed-ane",
		Host: &ane.Host{
			Instance: "projects/test-project/zones/us-central1-b/instances/test-node",
		},
		Status: &ane.Status{
			State: ane.StateActive,
		},
		// Aliases is nil/empty due to silent scrubbing
	}

	_, err := backend.GetNetworkInterfaces(ctx, testProviderID)
	if err == nil {
		t.Fatal("expected GetNetworkInterfaces to fail fast on scrubbed active ANE, got nil")
	}
	if !strings.Contains(err.Error(), "silent scrubbing") {
		t.Errorf("expected error to mention silent scrubbing, got: %v", err)
	}

	// Now insert an active endpoint that has an alias with an empty IP
	fakeANE.Endpoints["scrubbed-ane"].Aliases = map[string]*ane.Alias{
		ane.DefaultAliasName: {
			IPAddress: "",
		},
	}
	_, err = backend.GetNetworkInterfaces(ctx, testProviderID)
	if err == nil {
		t.Fatal("expected GetNetworkInterfaces to fail fast on empty IP active ANE, got nil")
	}
	if !strings.Contains(err.Error(), "empty IP address") {
		t.Errorf("expected error to mention empty IP address, got: %v", err)
	}
}
