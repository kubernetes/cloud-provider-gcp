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
	gce "k8s.io/cloud-provider-gcp/providers/gce"
	"k8s.io/utils/clock"
)

func TestNewPodIPBackendSelection(t *testing.T) {
	b1, err := newPodIPBackend(Options{}, nil)
	if err != nil {
		t.Fatalf("unexpected error for empty backend: %v", err)
	}
	if _, ok := b1.(*aliasRangesBackend); !ok {
		t.Fatalf("expected *aliasRangesBackend for empty backend, got %T", b1)
	}

	b2, err := newPodIPBackend(Options{PodIPBackend: BackendTypeAliasRanges}, nil)
	if err != nil {
		t.Fatalf("unexpected error for %q backend: %v", BackendTypeAliasRanges, err)
	}
	if _, ok := b2.(*aliasRangesBackend); !ok {
		t.Fatalf("expected *aliasRangesBackend for %q, got %T", BackendTypeAliasRanges, b2)
	}

	b3, err := newPodIPBackend(Options{PodIPBackend: BackendTypeANE}, nil)
	if err != nil {
		t.Fatalf("unexpected error for %q backend: %v", BackendTypeANE, err)
	}
	if _, ok := b3.(*aneBackend); !ok {
		t.Fatalf("expected *aneBackend for %q, got %T", BackendTypeANE, b3)
	}

	if _, err := newPodIPBackend(Options{PodIPBackend: "invalid-mode"}, nil); err == nil {
		t.Fatalf("expected error for unsupported backend type, got nil")
	}
}

func TestANEBackendGetNetworkInterfaces(t *testing.T) {
	ctx := context.Background()
	testClusterValues := gce.DefaultTestClusterValues()
	testClusterValues.ProjectID = testProject
	testClusterValues.ZoneName = testZone
	testClusterValues.NetworkURL = testNetworkURL
	fakeGCE := gce.NewFakeGCECloud(testClusterValues)

	backend := NewANEBackend(fakeGCE)

	// Add an active ANE
	err := fakeGCE.CreateAliasNetworkEndpoint(ctx, testProviderID, &gce.AliasNetworkEndpoint{
		Name:       "ane-active-1",
		Subnetwork: fakeGCE.SubnetworkURL(),
		Status:     &gce.ANEStatus{State: gce.ANEStateActive},
		Aliases: map[string]*gce.ANEAlias{
			gce.DefaultANEAliasName: {
				EffectiveIPAddress: "10.128.0.10",
			},
		},
	})
	if err != nil {
		t.Fatalf("Create active ANE: %v", err)
	}

	// Add a creating/inactive ANE (should be skipped)
	err = fakeGCE.CreateAliasNetworkEndpoint(ctx, testProviderID, &gce.AliasNetworkEndpoint{
		Name:       "ane-creating-2",
		Subnetwork: fakeGCE.SubnetworkURL(),
		Status:     &gce.ANEStatus{State: gce.ANEStateCreating},
		Aliases: map[string]*gce.ANEAlias{
			gce.DefaultANEAliasName: {
				EffectiveIPAddress: "10.128.0.20",
			},
		},
	})
	if err != nil {
		t.Fatalf("Create inactive ANE: %v", err)
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

	// Verify that an initial allocation on the VM's aliasIpRanges is
	// preserved alongside subsequent ANEs.
	mockInstances, ok := fakeGCE.Compute().BetaInstances().(*gcloud.MockBetaInstances)
	if !ok {
		t.Fatalf("Failed to cast BetaInstances to MockBetaInstances")
	}
	instanceKey := meta.ZonalKey(testNodeName, testZone)
	mockInstances.Objects[*instanceKey] = &gcloud.MockInstancesObj{
		Obj: &computebeta.Instance{
			Name: testNodeName,
			Zone: testZone,
			NetworkInterfaces: []*computebeta.NetworkInterface{
				{
					Name:       "nic0",
					Network:    testNetworkURL,
					Subnetwork: fakeGCE.SubnetworkURL(),
					AliasIpRanges: []*computebeta.AliasIpRange{
						{IpCidrRange: "10.100.0.0/28"},
					},
				},
			},
		},
	}

	ifacesWithInitial, err := backend.GetNetworkInterfaces(ctx, testProviderID)
	if err != nil {
		t.Fatalf("GetNetworkInterfaces with initial alias IP range: %v", err)
	}
	if len(ifacesWithInitial) != 1 || len(ifacesWithInitial[0].AliasIPRanges) != 2 {
		t.Fatalf("expected 2 alias IP ranges (1 initial VM range + 1 active ANE), got %v", ifacesWithInitial[0].AliasIPRanges)
	}
	if ifacesWithInitial[0].AliasIPRanges[0] != "10.100.0.0/28" || ifacesWithInitial[0].AliasIPRanges[1] != "10.128.0.10/32" {
		t.Fatalf("unexpected AliasIPRanges: %v", ifacesWithInitial[0].AliasIPRanges)
	}
}

func TestANEBackendMutateAdditionsAndRemovals(t *testing.T) {
	ctx := context.Background()
	testClusterValues := gce.DefaultTestClusterValues()
	testClusterValues.ProjectID = testProject
	testClusterValues.ZoneName = testZone
	testClusterValues.NetworkURL = testNetworkURL
	fakeGCE := gce.NewFakeGCECloud(testClusterValues)

	backend := NewANEBackend(fakeGCE)

	// Mutate: Add 2 ANEs
	err := backend.Mutate(ctx, testProviderID, testNetworkURL, []string{"/32", "/32"}, nil, nil)
	if err != nil {
		t.Fatalf("Mutate additions: %v", err)
	}

	anes, err := fakeGCE.ListAliasNetworkEndpoints(ctx, testProviderID)
	if err != nil {
		t.Fatalf("ListAliasNetworkEndpoints: %v", err)
	}
	if len(anes) != 2 {
		t.Fatalf("expected 2 endpoints in fakeGCE, got %d", len(anes))
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
	backend := NewANEBackend(fakeGCE)

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

	specCtrl := NewSpecController(
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
	anes, err := fakeGCE.ListAliasNetworkEndpoints(ctx, testProviderID)
	if err != nil {
		t.Fatalf("ListAliasNetworkEndpoints: %v", err)
	}
	if len(anes) != 2 {
		t.Fatalf("expected 2 ANEs created in fakeGCE, got %d", len(anes))
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

	// Now release 1 Pod IP: Pods requested goes down to 1, and 1 CIDR is
	// put in ReleasableCIDRs.
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

	// Verify 1 ANE remains in fakeGCE
	anesAfter, err := fakeGCE.ListAliasNetworkEndpoints(ctx, testProviderID)
	if err != nil {
		t.Fatalf("ListAliasNetworkEndpoints after release: %v", err)
	}
	if len(anesAfter) != 1 {
		t.Fatalf("expected 1 ANE remaining in fakeGCE, got %d", len(anesAfter))
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
			PodIPBackend:                 BackendTypeANE,
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

	_, _, started, err = StartControllers(
		ctx,
		Options{
			EnableDynamicPodIPController: false,
			PodIPBackend:                 BackendTypeANE,
		},
		kubeClient,
		nncClient,
		nodeInformer,
		fakeGCE,
	)
	if err == nil {
		t.Fatalf("expected error when PodIPBackend is set without EnableDynamicPodIPController")
	}
	if started {
		t.Fatalf("expected started == false when StartControllers returns an error")
	}
}

func TestCalculateAdditions(t *testing.T) {
	aliasRanges := NewAliasRangesBackend(nil)
	aneBackend := NewANEBackend(nil)

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
		"gce://test-project/us-central1-b/node-1.c.test-project.internal",
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

	backend := NewANEBackend(fakeGCE)

	// Removing an IP with no corresponding ANE in GCE should be a silent
	// no-op (no error).
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

	backend := NewANEBackend(fakeGCE)

	// Test concurrent addition of 16 ANEs (two batches under concurrency=8)
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

	backend := NewANEBackend(fakeGCE)

	// Mutate targeting netB -> should resolve subnetB for the ANE
	err := backend.Mutate(ctx, testProviderID, netB, []string{"/32"}, nil, nil)
	if err != nil {
		t.Fatalf("Mutate netB failed: %v", err)
	}

	anes, err := fakeGCE.ListAliasNetworkEndpoints(ctx, testProviderID)
	if err != nil {
		t.Fatalf("ListAliasNetworkEndpoints: %v", err)
	}
	var foundSubnetB bool
	for _, ep := range anes {
		if ep.Subnetwork == subnetB {
			foundSubnetB = true
			break
		}
	}
	if !foundSubnetB {
		t.Fatalf("expected ANE created with subnetwork %q, got: %v", subnetB, anes)
	}

	// Mutate targeting netA -> should resolve subnetA for the ANE
	err = backend.Mutate(ctx, testProviderID, netA, []string{"/32"}, nil, nil)
	if err != nil {
		t.Fatalf("Mutate netA failed: %v", err)
	}

	anes, err = fakeGCE.ListAliasNetworkEndpoints(ctx, testProviderID)
	if err != nil {
		t.Fatalf("ListAliasNetworkEndpoints: %v", err)
	}
	var foundSubnetA bool
	for _, ep := range anes {
		if ep.Subnetwork == subnetA {
			foundSubnetA = true
			break
		}
	}
	if !foundSubnetA {
		t.Fatalf("expected ANE created with subnetwork %q, got: %v", subnetA, anes)
	}

	// GetNetworkInterfaces should return both nic0 and nic1 with their
	// respective ANE IPs.
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
	fakeGCE := gce.NewFakeGCECloud(gce.DefaultTestClusterValues())
	backend := NewANEBackend(fakeGCE)

	testProviderID := "gce://test-project/us-central1-b/test-node"

	// Insert an active endpoint that has missing aliases (simulating silent
	// scrubbing).
	err := fakeGCE.CreateAliasNetworkEndpoint(ctx, testProviderID, &gce.AliasNetworkEndpoint{
		Name: "scrubbed-ane",
		Status: &gce.ANEStatus{
			State: gce.ANEStateActive,
		},
		// Aliases is nil/empty due to silent scrubbing
	})
	if err != nil {
		t.Fatalf("CreateAliasNetworkEndpoint failed: %v", err)
	}

	_, err = backend.GetNetworkInterfaces(ctx, testProviderID)
	if err == nil {
		t.Fatal("expected GetNetworkInterfaces to fail fast on scrubbed active ANE, got nil")
	}
	if !strings.Contains(err.Error(), "silent scrubbing") {
		t.Errorf("expected error to mention silent scrubbing, got: %v", err)
	}

	if err := fakeGCE.DeleteAliasNetworkEndpoint(ctx, testProviderID, "scrubbed-ane"); err != nil {
		t.Fatalf("DeleteAliasNetworkEndpoint failed: %v", err)
	}

	// Now insert an active endpoint that has an alias with an empty IP
	err = fakeGCE.CreateAliasNetworkEndpoint(ctx, testProviderID, &gce.AliasNetworkEndpoint{
		Name: "scrubbed-ane-empty-ip",
		Status: &gce.ANEStatus{
			State: gce.ANEStateActive,
		},
		Aliases: map[string]*gce.ANEAlias{
			gce.DefaultANEAliasName: {
				IPAddress: "",
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateAliasNetworkEndpoint failed: %v", err)
	}

	_, err = backend.GetNetworkInterfaces(ctx, testProviderID)
	if err == nil {
		t.Fatal("expected GetNetworkInterfaces to fail fast on empty IP active ANE, got nil")
	}
	if !strings.Contains(err.Error(), "empty IP address") {
		t.Errorf("expected error to mention empty IP address, got: %v", err)
	}
}
