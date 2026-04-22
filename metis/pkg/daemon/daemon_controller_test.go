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

package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	nncv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/nodenetworkconfig/v1"
	nncclientset "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/clientset/versioned"
	nnctypedv1 "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/clientset/versioned/typed/nodenetworkconfig/v1"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	adaptiveipam "k8s.io/metis/api/adaptiveipam/v1"
	"k8s.io/metis/pkg/store"
)

func TestDaemonController_Success(t *testing.T) {
	processed := make(map[string]bool)
	var mu sync.Mutex

	syncHandler := func(ctx context.Context, network string) error {
		mu.Lock()
		defer mu.Unlock()
		processed[network] = true
		return nil
	}

	logger := logr.Discard()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "metis_controller_success_test.sqlite")
	storeInstance, err := store.NewStore(context.Background(), logger, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer storeInstance.Close()

	c := NewDaemonController(DaemonControllerConfig{
		Name:   "test-controller",
		Logger: logger,
		Store:  storeInstance,
	})
	c.syncHandler = syncHandler
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go c.Run(ctx, 1)

	c.Enqueue("test-network")

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if !processed["test-network"] {
		t.Errorf("Expected test-network to be processed")
	}
}

func TestDaemonController_Retry(t *testing.T) {
	processCount := 0
	var mu sync.Mutex

	syncHandler := func(ctx context.Context, network string) error {
		mu.Lock()
		defer mu.Unlock()
		processCount++
		if processCount == 1 {
			return errors.New("temporary error")
		}
		return nil
	}

	logger := logr.Discard()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "metis_controller_retry_test.sqlite")
	storeInstance, err := store.NewStore(context.Background(), logger, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer storeInstance.Close()

	c := NewDaemonController(DaemonControllerConfig{
		Name:   "test-controller-retry",
		Logger: logger,
		Store:  storeInstance,
	})
	c.syncHandler = syncHandler
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go c.Run(ctx, 1)

	c.Enqueue("test-network")

	// Wait for processing and retry
	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if processCount < 2 {
		t.Errorf("Expected at least 2 attempts, got %d", processCount)
	}
}

type mockNodeNetworkConfigInterface struct {
	nnctypedv1.NodeNetworkConfigInterface
	getFunc    func(ctx context.Context, name string, opts metav1.GetOptions) (*nncv1.NodeNetworkConfig, error)
	updateFunc func(ctx context.Context, nnc *nncv1.NodeNetworkConfig, opts metav1.UpdateOptions) (*nncv1.NodeNetworkConfig, error)
	patchFunc  func(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (*nncv1.NodeNetworkConfig, error)
}

func (m *mockNodeNetworkConfigInterface) Get(ctx context.Context, name string, opts metav1.GetOptions) (*nncv1.NodeNetworkConfig, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, name, opts)
	}
	return nil, nil
}

func (m *mockNodeNetworkConfigInterface) Update(ctx context.Context, nnc *nncv1.NodeNetworkConfig, opts metav1.UpdateOptions) (*nncv1.NodeNetworkConfig, error) {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, nnc, opts)
	}
	return nil, nil
}

func (m *mockNodeNetworkConfigInterface) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (*nncv1.NodeNetworkConfig, error) {
	if m.patchFunc != nil {
		return m.patchFunc(ctx, name, pt, data, opts, subresources...)
	}
	return nil, nil
}

type mockNetworkingV1 struct {
	nnctypedv1.NetworkingV1Interface
	nncInterface nnctypedv1.NodeNetworkConfigInterface
}

func (m *mockNetworkingV1) NodeNetworkConfigs() nnctypedv1.NodeNetworkConfigInterface {
	return m.nncInterface
}

type mockClientset struct {
	nncclientset.Interface
	networkingV1 nnctypedv1.NetworkingV1Interface
}

func (m *mockClientset) NetworkingV1() nnctypedv1.NetworkingV1Interface {
	return m.networkingV1
}

func TestDaemonController_DynamicAllocation_HighUtilization(t *testing.T) {
	logger := klog.Background()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "metis_controller_test.sqlite")

	storeInstance, err := store.NewStore(context.Background(), logger, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer storeInstance.Close()

	network := "test-network"
	nodeName := "test-node"

	// Add initial CIDR to DB
	err = storeInstance.AddCIDR(context.Background(), network, "10.0.1.0/28") // 16 IPs
	if err != nil {
		t.Fatalf("Failed to add CIDR: %v", err)
	}

	// Add another CIDR to make it 48 total.
	err = storeInstance.AddCIDR(context.Background(), network, "10.0.2.0/27") // 32 IPs
	if err != nil {
		t.Fatalf("Failed to add CIDR: %v", err)
	}

	// Allocate 42 IPs to trigger high utilization (>80% of 48)
	for i := 0; i < 42; i++ {
		_, _, err = storeInstance.AllocateIPv4(context.Background(), network, "eth0", fmt.Sprintf("container-%d", i))
		if err != nil {
			t.Fatalf("Failed to allocate IP: %v", err)
		}
	}

	mockNNC := &nncv1.NodeNetworkConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
		},
		Spec: nncv1.NodeNetworkConfigSpec{
			Allocations: []nncv1.Allocation{
				{
					Network: network,
					Pods:    32,
				},
			},
		},
		Status: nncv1.NodeNetworkConfigStatus{
			PodCIDRs: []nncv1.PodCIDR{
				{CIDR: "10.0.2.0/27", Network: network},
			},
		},
	}

	patchCalled := false
	var patchedData []byte

	mockInterface := &mockNodeNetworkConfigInterface{
		getFunc: func(ctx context.Context, name string, opts metav1.GetOptions) (*nncv1.NodeNetworkConfig, error) {
			return mockNNC, nil
		},
		patchFunc: func(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (*nncv1.NodeNetworkConfig, error) {
			patchCalled = true
			patchedData = data
			return mockNNC, nil
		},
	}

	mockNetV1 := &mockNetworkingV1{nncInterface: mockInterface}
	mockClient := &mockClientset{networkingV1: mockNetV1}

	c := NewDaemonController(DaemonControllerConfig{
		Name:                    "test-controller",
		Logger:                  logger,
		NNCClient:               mockClient,
		NodeName:                nodeName,
		Store:                   storeInstance,
		GetPendingRequestsCount: func(net string) int { return 10 },
	})

	err = c.dynamicAllocation(context.Background(), network)
	if err != nil {
		t.Fatalf("dynamicAllocation failed: %v", err)
	}

	if !patchCalled {
		t.Fatal("Expected patch to be called")
	}

	if patchedData == nil {
		t.Fatal("Expected patchedData to be non-nil")
	}

	var patch struct {
		Spec nncv1.NodeNetworkConfigSpec `json:"spec"`
	}
	err = json.Unmarshal(patchedData, &patch)
	if err != nil {
		t.Fatalf("Failed to unmarshal patch data: %v", err)
	}

	if len(patch.Spec.Allocations) == 0 {
		t.Fatal("Expected allocations to be non-empty")
	}

	// We expect 58 pods because:
	// Used IPs = 45 (42 allocated + 3 reserved in initial block).
	// Pending Requests = 10.
	// Total targeted used = 55.
	// To bring utilization under 75%: 55 / 0.75 = 73.33 -> 74 total capacity needed.
	// Subtracting initial IPs (16): 74 - 16 = 58.
	if patch.Spec.Allocations[0].Pods != 58 {
		t.Errorf("Expected new target pods to be 58, got %d", patch.Spec.Allocations[0].Pods)
	}
}

func TestDaemonController_DynamicAllocation_NoOp_ZeroInitialIPs(t *testing.T) {
	logger := klog.Background()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "metis_controller_zero_initial_test.sqlite")

	storeInstance, err := store.NewStore(context.Background(), logger, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer storeInstance.Close()

	network := "test-network"
	nodeName := "test-node"

	// Do NOT add initial CIDR to DB, so initialIPs will be 0.

	mockNNC := &nncv1.NodeNetworkConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
		},
		Spec: nncv1.NodeNetworkConfigSpec{
			Allocations: []nncv1.Allocation{
				{
					Network: network,
					Pods:    32,
				},
			},
		},
		Status: nncv1.NodeNetworkConfigStatus{
			PodCIDRs: []nncv1.PodCIDR{},
		},
	}

	patchCalled := false

	mockInterface := &mockNodeNetworkConfigInterface{
		getFunc: func(ctx context.Context, name string, opts metav1.GetOptions) (*nncv1.NodeNetworkConfig, error) {
			return mockNNC, nil
		},
		patchFunc: func(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (*nncv1.NodeNetworkConfig, error) {
			patchCalled = true
			return mockNNC, nil
		},
	}

	mockNetV1 := &mockNetworkingV1{nncInterface: mockInterface}
	mockClient := &mockClientset{networkingV1: mockNetV1}

	c := NewDaemonController(DaemonControllerConfig{
		Name:                    "test-controller",
		Logger:                  logger,
		NNCClient:               mockClient,
		NodeName:                nodeName,
		Store:                   storeInstance,
		GetPendingRequestsCount: func(net string) int { return 10 },
	})

	err = c.dynamicAllocation(context.Background(), network)
	if err != nil {
		t.Fatalf("dynamicAllocation failed: %v", err)
	}

	if patchCalled {
		t.Error("Expected patch NOT to be called when initial IPs is 0")
	}
}

func TestDaemonController_DynamicAllocation_ExcludeDrainingFromExhaustion(t *testing.T) {
	logger := logr.Discard()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "metis_controller_draining_test.sqlite")

	storeInstance, err := store.NewStore(context.Background(), logger, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer storeInstance.Close()

	network := "test-network"
	nodeName := "test-node"

	// Add initial block (16 IPs)
	err = storeInstance.AddCIDR(context.Background(), network, "10.0.1.0/28")
	if err != nil {
		t.Fatalf("Failed to add CIDR: %v", err)
	}

	// Add block 2 (Ready, 32 IPs)
	err = storeInstance.AddCIDR(context.Background(), network, "10.0.2.0/27")
	if err != nil {
		t.Fatalf("Failed to add CIDR: %v", err)
	}

	// Add block 3 (Draining, 32 IPs)
	err = storeInstance.AddCIDR(context.Background(), network, "10.0.3.0/27")
	if err != nil {
		t.Fatalf("Failed to add CIDR: %v", err)
	}
	block3, err := storeInstance.GetReadyCIDRBlocksSorted(context.Background(), network)
	if err != nil || len(block3) == 0 {
		t.Fatalf("Failed to get ready blocks: %v", err)
	}
	err = storeInstance.DrainCIDRBlock(context.Background(), block3[0].ID) // Assuming block 3 is latest
	if err != nil {
		t.Fatalf("DrainCIDRBlock failed: %v", err)
	}

	// Total capacity in DB (Ready + Draining) = 16 + 32 + 32 = 80.
	
	// Allocate 40 IPs (to simulate some usage)
	for i := 0; i < 40; i++ {
		_, _, err = storeInstance.AllocateIPv4(context.Background(), network, "eth0", fmt.Sprintf("container-%d", i))
		if err != nil {
			t.Fatalf("Failed to allocate IP: %v", err)
		}
	}

	// Pending requests = 10.
	// Total targeted used = 40 + 10 = 50.

	// Correct formula: Utilization = (used + pending) / totalRequestedCapacity
	// Utilization = (40 + 10) / (16 + 64) = 50 / 80 = 0.625 < 0.80.
	// Scale-up should NOT be triggered!
	//
	// Buggy formula (if we added draining capacity to used):
	// Utilization = (40 + 10 + 32) / 80 = 82 / 80 = 1.025 > 0.80.
	// Scale-up WOULD be triggered!

	// Setup mock CRD
	mockNNC := &nncv1.NodeNetworkConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
		},
		Spec: nncv1.NodeNetworkConfigSpec{
			Allocations: []nncv1.Allocation{
				{
					Network: network,
					Pods:    64, // Set to 64 so totalRequestedCapacity = 16 + 64 = 80
				},
			},
		},
		Status: nncv1.NodeNetworkConfigStatus{
			PodCIDRs: []nncv1.PodCIDR{
				{CIDR: "10.0.2.0/27", Network: network},
			},
		},
	}

	patchCalled := false
	mockInterface := &mockNodeNetworkConfigInterface{
		getFunc: func(ctx context.Context, name string, opts metav1.GetOptions) (*nncv1.NodeNetworkConfig, error) {
			return mockNNC, nil
		},
		patchFunc: func(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (*nncv1.NodeNetworkConfig, error) {
			patchCalled = true
			return mockNNC, nil
		},
	}

	mockNetV1 := &mockNetworkingV1{nncInterface: mockInterface}
	mockClient := &mockClientset{networkingV1: mockNetV1}

	c := NewDaemonController(DaemonControllerConfig{
		Name:                    "test-controller",
		Logger:                  logger,
		NNCClient:               mockClient,
		NodeName:                nodeName,
		Store:                   storeInstance,
		GetPendingRequestsCount: func(net string) int { return 10 },
	})

	err = c.dynamicAllocation(context.Background(), network)
	if err != nil {
		t.Fatalf("dynamicAllocation failed: %v", err)
	}

	if patchCalled {
		t.Error("Expected patch NOT to be called, but it was called")
	}
}

func TestDaemonController_DynamicAllocation_HighUtilization_CooldownPushback(t *testing.T) {
	logger := logr.Discard()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "metis_controller_cooldown_test.sqlite")

	storeInstance, err := store.NewStore(context.Background(), logger, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer storeInstance.Close()

	network := "test-network"
	nodeName := "test-node"

	// Add initial block (16 IPs)
	err = storeInstance.AddCIDR(context.Background(), network, "10.0.1.0/28")
	if err != nil {
		t.Fatalf("Failed to add CIDR: %v", err)
	}

	// Add block 2 (128 IPs) to make it 144 total capacity
	err = storeInstance.AddCIDR(context.Background(), network, "10.0.2.0/25")
	if err != nil {
		t.Fatalf("Failed to add CIDR: %v", err)
	}

	// Allocate 11 IPs and release them to create cooldown state
	for i := 0; i < 11; i++ {
		_, _, err = storeInstance.AllocateIPv4(context.Background(), network, "eth0", fmt.Sprintf("cooldown-container-%d", i))
		if err != nil {
			t.Fatalf("Failed to allocate IP: %v", err)
		}
	}
	
	for i := 0; i < 11; i++ {
		_, err = storeInstance.ReleaseIPByOwner(context.Background(), network, fmt.Sprintf("cooldown-container-%d", i), "eth0", 1*time.Hour)
		if err != nil {
			t.Fatalf("Failed to release IP: %v", err)
		}
	}

	// Allocate 60 IPs to simulate high usage
	for i := 0; i < 60; i++ {
		_, _, err = storeInstance.AllocateIPv4(context.Background(), network, "eth0", fmt.Sprintf("container-%d", i))
		if err != nil {
			t.Fatalf("Failed to allocate IP: %v", err)
		}
	}

	// Setup mock CRD
	mockNNC := &nncv1.NodeNetworkConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
		},
		Spec: nncv1.NodeNetworkConfigSpec{
			Allocations: []nncv1.Allocation{
				{
					Network: network,
					Pods:    64, // totalRequestedCapacity = 16 + 64 = 80
				},
			},
		},
		Status: nncv1.NodeNetworkConfigStatus{
			PodCIDRs: []nncv1.PodCIDR{
				{CIDR: "10.0.2.0/25", Network: network},
			},
		},
	}

	patchCalled := false
	mockInterface := &mockNodeNetworkConfigInterface{
		getFunc: func(ctx context.Context, name string, opts metav1.GetOptions) (*nncv1.NodeNetworkConfig, error) {
			return mockNNC, nil
		},
		patchFunc: func(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (*nncv1.NodeNetworkConfig, error) {
			patchCalled = true
			return mockNNC, nil
		},
	}

	mockNetV1 := &mockNetworkingV1{nncInterface: mockInterface}
	mockClient := &mockClientset{networkingV1: mockNetV1}

	c := NewDaemonController(DaemonControllerConfig{
		Name:                     "test-controller",
		Logger:                   logger,
		NNCClient:                mockClient,
		NodeName:                 nodeName,
		Store:                    storeInstance,
		GetPendingRequestsCount:  func(net string) int { return 10 },
		CooldownPushbackInterval: 1 * time.Millisecond,
	})

	err = c.dynamicAllocation(context.Background(), network)
	if err != nil {
		t.Fatalf("dynamicAllocation failed: %v", err)
	}

	if patchCalled {
		t.Error("Expected patch NOT to be called due to cooldown pushback, but it was called")
	}

	// Wait for the item to be added to the queue (it has 1ms delay)
	time.Sleep(10 * time.Millisecond)

	if c.queue.Len() != 1 {
		t.Errorf("Expected 1 item in queue, got %d", c.queue.Len())
	}

	item, quit := c.queue.Get()
	if quit {
		t.Fatal("Queue quit unexpectedly")
	}
	if item != network {
		t.Errorf("Expected item %s in queue, got %s", network, item)
	}
}

func TestDaemonController_SyncCidr_Add(t *testing.T) {
	logger := logr.Discard()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "metis_controller_sync_cidr_add_test.sqlite")
	storeInstance, err := store.NewStore(context.Background(), logger, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer storeInstance.Close()

	network := "test-network"
	nodeName := "test-node"
	cidr := "10.0.1.0/28"

	mockNNC := &nncv1.NodeNetworkConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
		},
		Status: nncv1.NodeNetworkConfigStatus{
			PodCIDRs: []nncv1.PodCIDR{
				{
					CIDR:    cidr,
					Network: network,
					Condition: &metav1.Condition{
						Status: metav1.ConditionTrue,
					},
				},
			},
		},
	}

	mockInterface := &mockNodeNetworkConfigInterface{
		getFunc: func(ctx context.Context, name string, opts metav1.GetOptions) (*nncv1.NodeNetworkConfig, error) {
			return mockNNC, nil
		},
	}
	mockNetV1 := &mockNetworkingV1{nncInterface: mockInterface}
	mockClient := &mockClientset{networkingV1: mockNetV1}

	c := NewDaemonController(DaemonControllerConfig{
		Name:      "test-controller",
		Logger:    logger,
		NNCClient: mockClient,
		NodeName:  nodeName,
		Store:     storeInstance,
	})

	err = c.syncCIDR(context.Background(), network)
	if err != nil {
		t.Fatalf("syncCIDR failed: %v", err)
	}

	// Verify DB
	exists, err := storeInstance.GetCIDRBlockByCIDR(context.Background(), cidr)
	if err != nil {
		t.Fatalf("GetCIDRBlockByCIDR failed: %v", err)
	}
	if !exists {
		t.Errorf("Expected CIDR block %s to exist in DB", cidr)
	}
}

func TestDaemonController_SyncCidr_IgnoreUnready(t *testing.T) {
	logger := logr.Discard()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "metis_controller_sync_cidr_ignore_unready_test.sqlite")
	storeInstance, err := store.NewStore(context.Background(), logger, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer storeInstance.Close()

	network := "test-network"
	nodeName := "test-node"
	cidr := "10.0.1.0/28"

	mockNNC := &nncv1.NodeNetworkConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
		},
		Status: nncv1.NodeNetworkConfigStatus{
			PodCIDRs: []nncv1.PodCIDR{
				{
					CIDR:    cidr,
					Network: network,
					Condition: &metav1.Condition{
						Status: metav1.ConditionFalse,
					},
				},
			},
		},
	}

	mockInterface := &mockNodeNetworkConfigInterface{
		getFunc: func(ctx context.Context, name string, opts metav1.GetOptions) (*nncv1.NodeNetworkConfig, error) {
			return mockNNC, nil
		},
	}
	mockNetV1 := &mockNetworkingV1{nncInterface: mockInterface}
	mockClient := &mockClientset{networkingV1: mockNetV1}

	c := NewDaemonController(DaemonControllerConfig{
		Name:      "test-controller",
		Logger:    logger,
		NNCClient: mockClient,
		NodeName:  nodeName,
		Store:     storeInstance,
	})

	err = c.syncCIDR(context.Background(), network)
	if err != nil {
		t.Fatalf("syncCIDR failed: %v", err)
	}

	// Verify DB
	exists, err := storeInstance.GetCIDRBlockByCIDR(context.Background(), cidr)
	if err != nil {
		t.Fatalf("GetCIDRBlockByCIDR failed: %v", err)
	}
	if exists {
		t.Errorf("Expected CIDR block %s to NOT exist in DB", cidr)
	}
}

func TestDaemonController_SyncCidr_Cleanup(t *testing.T) {
	logger := logr.Discard()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "metis_controller_sync_cidr_cleanup_test.sqlite")
	storeInstance, err := store.NewStore(context.Background(), logger, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer storeInstance.Close()

	network := "test-network"
	nodeName := "test-node"
	cidr := "10.0.1.0/28"

	// Add CIDR to DB and mark as Deleting
	err = storeInstance.AddCIDR(context.Background(), network, cidr)
	if err != nil {
		t.Fatalf("Failed to add CIDR: %v", err)
	}
	
	block, err := storeInstance.GetReadyCIDRBlocksSorted(context.Background(), network)
	if err != nil || len(block) == 0 {
		t.Fatalf("Failed to get block ID: %v", err)
	}
	err = storeInstance.MarkCIDRBlockAsDeleting(context.Background(), block[0].ID)
	if err != nil {
		t.Fatalf("Failed to mark block as Deleting: %v", err)
	}

	mockNNC := &nncv1.NodeNetworkConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
		},
		Spec: nncv1.NodeNetworkConfigSpec{
			ReleasableCIDRs: []nncv1.PodCIDR{}, // Empty
		},
	}

	mockInterface := &mockNodeNetworkConfigInterface{
		getFunc: func(ctx context.Context, name string, opts metav1.GetOptions) (*nncv1.NodeNetworkConfig, error) {
			return mockNNC, nil
		},
	}
	mockNetV1 := &mockNetworkingV1{nncInterface: mockInterface}
	mockClient := &mockClientset{networkingV1: mockNetV1}

	c := NewDaemonController(DaemonControllerConfig{
		Name:      "test-controller",
		Logger:    logger,
		NNCClient: mockClient,
		NodeName:  nodeName,
		Store:     storeInstance,
	})

	err = c.syncCIDR(context.Background(), network)
	if err != nil {
		t.Fatalf("syncCIDR failed: %v", err)
	}

	// Verify DB
	exists, err := storeInstance.GetCIDRBlockByCIDR(context.Background(), cidr)
	if err != nil {
		t.Fatalf("GetCIDRBlockByCIDR failed: %v", err)
	}
	if exists {
		t.Errorf("Expected CIDR block %s to be deleted from DB", cidr)
	}
}

func TestDaemonController_DynamicAllocation_LowUtilization(t *testing.T) {
	logger := logr.Discard()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "metis_controller_low_utilization_test.sqlite")
	storeInstance, err := store.NewStore(context.Background(), logger, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer storeInstance.Close()

	network := "test-network"
	nodeName := "test-node"

	// Add initial block (protect this)
	err = storeInstance.AddCIDR(context.Background(), network, "10.0.1.0/28") // 16 IPs
	if err != nil {
		t.Fatalf("Failed to add CIDR: %v", err)
	}

	// Add block 2 (should be drained)
	err = storeInstance.AddCIDR(context.Background(), network, "10.0.2.0/27") // 32 IPs
	if err != nil {
		t.Fatalf("Failed to add CIDR: %v", err)
	}

	// Add block 3 (should be drained first because it's latest)
	err = storeInstance.AddCIDR(context.Background(), network, "10.0.3.0/27") // 32 IPs
	if err != nil {
		t.Fatalf("Failed to add CIDR: %v", err)
	}

	mockNNC := &nncv1.NodeNetworkConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
		},
		Spec: nncv1.NodeNetworkConfigSpec{
			Allocations: []nncv1.Allocation{
				{
					Network: network,
					Pods:    50,
				},
			},
		},
	}

	mockInterface := &mockNodeNetworkConfigInterface{
		getFunc: func(ctx context.Context, name string, opts metav1.GetOptions) (*nncv1.NodeNetworkConfig, error) {
			return mockNNC, nil
		},
	}
	mockNetV1 := &mockNetworkingV1{nncInterface: mockInterface}
	mockClient := &mockClientset{networkingV1: mockNetV1}

	c := NewDaemonController(DaemonControllerConfig{
		Name:      "test-controller",
		Logger:    logger,
		NNCClient: mockClient,
		NodeName:  nodeName,
		Store:     storeInstance,
	})

	// Manually set the timer to 9 hours ago
	c.lowUtilizationTimers[network] = time.Now().Add(-9 * time.Hour)

	err = c.dynamicAllocation(context.Background(), network)
	if err != nil {
		t.Fatalf("dynamicAllocation failed: %v", err)
	}

	// Verify DB
	readyBlocks, err := storeInstance.GetReadyCIDRBlocksSorted(context.Background(), network)
	if err != nil {
		t.Fatalf("GetReadyCIDRBlocksSorted failed: %v", err)
	}

	if len(readyBlocks) != 2 {
		t.Errorf("Expected 2 ready blocks, got %d", len(readyBlocks))
	}

	for _, b := range readyBlocks {
		if b.CIDR == "10.0.3.0/27" {
			t.Errorf("Expected block %s to be Draining, but it is still Ready", b.CIDR)
		}
	}
}

func TestDaemonController_FullLoop(t *testing.T) {
	logger := logr.Discard()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "metis_controller_full_loop_test.sqlite")
	storeInstance, err := store.NewStore(context.Background(), logger, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer storeInstance.Close()

	network := "test-network"
	nodeName := "test-node"

	// Add initial block
	err = storeInstance.AddCIDR(context.Background(), network, "10.0.1.0/28")
	if err != nil {
		t.Fatalf("Failed to add CIDR: %v", err)
	}

	mockNNC := &nncv1.NodeNetworkConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
		},
		Spec: nncv1.NodeNetworkConfigSpec{
			Allocations: []nncv1.Allocation{
				{Network: network, Pods: 0},
			},
		},
		Status: nncv1.NodeNetworkConfigStatus{
			PodCIDRs: []nncv1.PodCIDR{
				{CIDR: "10.0.1.0/28", Network: network},
			},
		},
	}

	var mu sync.Mutex
	
	mockInterface := &mockNodeNetworkConfigInterface{
		getFunc: func(ctx context.Context, name string, opts metav1.GetOptions) (*nncv1.NodeNetworkConfig, error) {
			mu.Lock()
			defer mu.Unlock()
			return mockNNC.DeepCopy(), nil
		},
		patchFunc: func(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (*nncv1.NodeNetworkConfig, error) {
			mu.Lock()
			defer mu.Unlock()
			
			var patch struct {
				Spec nncv1.NodeNetworkConfigSpec `json:"spec"`
			}
			err := json.Unmarshal(data, &patch)
			if err != nil {
				return nil, err
			}
			
			mockNNC.Spec.Allocations = patch.Spec.Allocations
			
			for _, alloc := range mockNNC.Spec.Allocations {
				if alloc.Pods > 0 && len(mockNNC.Status.PodCIDRs) == 1 {
					mockNNC.Status.PodCIDRs = append(mockNNC.Status.PodCIDRs, nncv1.PodCIDR{
						Id:      "block-2",
						Network: network,
						CIDR:    "10.0.2.0/28",
						Condition: &metav1.Condition{
							Status: metav1.ConditionTrue,
						},
					})
				}
			}
			
			return mockNNC, nil
		},
	}

	mockNetV1 := &mockNetworkingV1{nncInterface: mockInterface}
	mockClient := &mockClientset{networkingV1: mockNetV1}

	c := NewDaemonController(DaemonControllerConfig{
		Name:      "test-controller",
		Logger:    logger,
		NNCClient: mockClient,
		NodeName:  nodeName,
		Store:     storeInstance,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	go c.Run(ctx, 1)

	c.GetPendingRequestsCount = func(net string) int {
		return 15
	}

	c.Enqueue(network)

	time.Sleep(500 * time.Millisecond)

	c.Enqueue(network)
	
	time.Sleep(500 * time.Millisecond)

	exists, err := storeInstance.GetCIDRBlockByCIDR(context.Background(), "10.0.2.0/28")
	if err != nil {
		t.Fatalf("GetCIDRBlockByCIDR failed: %v", err)
	}
	if !exists {
		t.Errorf("Expected new CIDR block 10.0.2.0/28 to exist in DB")
	}
}

func TestDaemonController_ConcurrentAllocationAndDraining(t *testing.T) {
	logger := logr.Discard()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "metis_controller_concurrent_test.sqlite")
	storeInstance, err := store.NewStore(context.Background(), logger, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer storeInstance.Close()

	network := "test-network"
	nodeName := "test-node"

	// Add initial block
	err = storeInstance.AddCIDR(context.Background(), network, "10.0.1.0/28") // 16 IPs
	if err != nil {
		t.Fatalf("Failed to add CIDR: %v", err)
	}
	// Add block 2
	err = storeInstance.AddCIDR(context.Background(), network, "10.0.2.0/27") // 32 IPs
	if err != nil {
		t.Fatalf("Failed to add CIDR: %v", err)
	}

	server := newAdaptiveIpamServer(logger, storeInstance, "", 0, 0)
	c := NewDaemonController(DaemonControllerConfig{
		Name:      "test-controller",
		Logger:    logger,
		NodeName:  nodeName,
		Store:     storeInstance,
	})

	// Set timer for low utilization
	c.lowUtilizationTimers[network] = time.Now().Add(-9 * time.Hour)

	// Run dynamicAllocation in a loop
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				c.dynamicAllocation(ctx, network)
				time.Sleep(10 * time.Millisecond)
			}
		}
	}()

	// Run concurrent allocations
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			req := &adaptiveipam.AllocatePodIPRequest{
				Network:      network,
				PodName:      fmt.Sprintf("pod-%d", id),
				PodNamespace: "default",
				Ipv4Config: &adaptiveipam.IPConfig{
					InterfaceName: "eth0",
					ContainerId:   fmt.Sprintf("container-%d", id),
				},
			}
			_, err := server.AllocatePodIP(ctx, req)
			if err != nil {
				t.Errorf("Allocation failed for pod-%d: %v", id, err)
			}
		}(i)
	}

	wg.Wait()

	// Verify DB state
	readyBlocks, err := storeInstance.GetReadyCIDRBlocksSorted(context.Background(), network)
	if err != nil {
		t.Fatalf("GetReadyCIDRBlocksSorted failed: %v", err)
	}
	
	if len(readyBlocks) == 0 {
		t.Errorf("Expected at least the initial block to be Ready")
	}
}

func TestDaemonController_MaybeDrainExcessive(t *testing.T) {
	logger := logr.Discard()
	
	network := "test-network"
	nodeName := "test-node"

	ctx := context.Background()

	tests := []struct {
		desc                    string
		setTimer                bool
		timerDuration           time.Duration
		utilization             float64
		initialIPs              int
		targetPods              int
		usage                   store.NetworkIPUsage
		blocksToAdd             []string
		expectedTimerExists     bool
		expectedTimerUnchanged  bool
		expectedDrained         bool
	}{
		{
			desc: "Utilization above threshold resets timer",
			setTimer: true,
			timerDuration: 0,
			utilization: lowUtilizationThreshold + 0.1,
			expectedTimerExists: false,
		},
		{
			desc: "Utilization above threshold no-op when no timer",
			setTimer: false,
			utilization: lowUtilizationThreshold + 0.1,
			expectedTimerExists: false,
		},
		{
			desc: "Utilization below threshold starts timer",
			setTimer: false,
			utilization: lowUtilizationThreshold - 0.1,
			expectedTimerExists: true,
		},
		{
			desc: "Utilization below threshold maintains timer",
			setTimer: true,
			timerDuration: -1 * time.Hour,
			utilization: lowUtilizationThreshold - 0.1,
			expectedTimerExists: true,
			expectedTimerUnchanged: true,
		},
		{
			desc: "Utilization below threshold for sustained duration triggers drain",
			setTimer: true,
			timerDuration: -9 * time.Hour,
			utilization: lowUtilizationThreshold - 0.1,
			initialIPs: 16,
			targetPods: 16,
			usage: store.NetworkIPUsage{Total: 32},
			blocksToAdd: []string{"10.0.1.0/28", "10.0.2.0/28"},
			expectedTimerExists: false,
			expectedDrained: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			var storeInstance *store.Store
			var err error
			if len(tc.blocksToAdd) > 0 {
				tempDir := t.TempDir()
				dbPath := filepath.Join(tempDir, "metis_controller_table_test.sqlite")
				storeInstance, err = store.NewStore(context.Background(), logger, dbPath)
				if err != nil {
					t.Fatalf("Failed to create store: %v", err)
				}
				defer storeInstance.Close()

				for _, cidr := range tc.blocksToAdd {
					err = storeInstance.AddCIDR(ctx, network, cidr)
					if err != nil {
						t.Fatalf("Failed to add CIDR: %v", err)
					}
				}
			}

			c := NewDaemonController(DaemonControllerConfig{
				Name:     "test-controller",
				Logger:   logger,
				NodeName: nodeName,
				Store:    storeInstance,
			})

			var startTime time.Time
			if tc.setTimer {
				startTime = time.Now().Add(tc.timerDuration)
				c.lowUtilizationTimers[network] = startTime
			}

			info := &UtilizationInfo{
				Utilization: tc.utilization,
				InitialIPs:  tc.initialIPs,
				TargetPods:  tc.targetPods,
				Usage:       tc.usage,
			}

			drained := c.maybeDrainExcessive(ctx, network, info)

			if drained != tc.expectedDrained {
				t.Errorf("Expected drained %v, got %v", tc.expectedDrained, drained)
			}

			timer, ok := c.lowUtilizationTimers[network]
			if ok != tc.expectedTimerExists {
				t.Errorf("Expected timer exists %v, got %v", tc.expectedTimerExists, ok)
			}

			if ok && tc.expectedTimerUnchanged && !timer.Equal(startTime) {
				t.Errorf("Expected timer to be unchanged, but it was modified")
			}
		})
	}
}

func TestDaemonController_HandleExpiredDrainingBlocks(t *testing.T) {
	logger := klog.Background()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "metis_controller_indirect_test.sqlite")
	storeInstance, err := store.NewStore(context.Background(), logger, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer storeInstance.Close()

	network := "test-network"
	nodeName := "test-node"

	// Add initial block
	err = storeInstance.AddCIDR(context.Background(), network, "10.0.1.0/28")
	if err != nil {
		t.Fatalf("Failed to add CIDR: %v", err)
	}

	// Add another block
	err = storeInstance.AddCIDR(context.Background(), network, "10.0.2.0/28")
	if err != nil {
		t.Fatalf("Failed to add CIDR: %v", err)
	}

	readyBlocks, err := storeInstance.GetReadyCIDRBlocksSorted(context.Background(), network)
	if err != nil || len(readyBlocks) < 2 {
		t.Fatalf("Failed to get ready blocks: %v", err)
	}

	// Mark the second block as Draining
	err = storeInstance.DrainCIDRBlock(context.Background(), readyBlocks[0].ID) // Assuming readyBlocks[0] is not initial
	if err != nil {
		t.Fatalf("DrainCIDRBlock failed: %v", err)
	}

	// Wait for it to expire (we set DrainingExpiration to 1s in config)
	time.Sleep(1 * time.Second)

	mockNNC := &nncv1.NodeNetworkConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
		},
		Spec: nncv1.NodeNetworkConfigSpec{
			Allocations: []nncv1.Allocation{
				{Network: network, Pods: 16},
			},
		},
		Status: nncv1.NodeNetworkConfigStatus{
			PodCIDRs: []nncv1.PodCIDR{
				{Id: "block-2", CIDR: "10.0.2.0/28", Network: network},
			},
		},
	}

	var patchCount int
	var patchErr error
	var patchedData []byte

	mockInterface := &mockNodeNetworkConfigInterface{
		getFunc: func(ctx context.Context, name string, opts metav1.GetOptions) (*nncv1.NodeNetworkConfig, error) {
			return mockNNC, nil
		},
		patchFunc: func(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (*nncv1.NodeNetworkConfig, error) {
			patchCount++
			patchedData = data
			if patchErr != nil {
				return nil, patchErr
			}
			// Update mockNNC spec for subsequent calls
			var patch struct {
				Spec nncv1.NodeNetworkConfigSpec `json:"spec"`
			}
			json.Unmarshal(data, &patch)
			mockNNC.Spec.ReleasableCIDRs = patch.Spec.ReleasableCIDRs
			return mockNNC, nil
		},
	}

	mockNetV1 := &mockNetworkingV1{nncInterface: mockInterface}
	mockClient := &mockClientset{networkingV1: mockNetV1}

	c := NewDaemonController(DaemonControllerConfig{
		Name:               "test-controller",
		Logger:             logger,
		NNCClient:          mockClient,
		NodeName:           nodeName,
		Store:              storeInstance,
		DrainingExpiration: 1 * time.Second,
	})

	t.Run("Success scenario adds to ReleasableCIDRs", func(t *testing.T) {
		patchCount = 0
		patchErr = nil
		mockNNC.Spec.ReleasableCIDRs = []nncv1.PodCIDR{} // Reset

		err = c.dynamicAllocation(context.Background(), network)
		if err != nil {
			t.Fatalf("dynamicAllocation failed: %v", err)
		}

		if patchCount != 1 {
			t.Errorf("Expected patch to be called once, got %d", patchCount)
		}

		var patchData struct {
			Spec nncv1.NodeNetworkConfigSpec `json:"spec"`
		}
		json.Unmarshal(patchedData, &patchData)

		if len(patchData.Spec.ReleasableCIDRs) != 1 {
			t.Errorf("Expected 1 releasable CIDR in patch, got %d", len(patchData.Spec.ReleasableCIDRs))
		} else if patchData.Spec.ReleasableCIDRs[0].CIDR != "10.0.2.0/28" {
			t.Errorf("Expected releasable CIDR 10.0.2.0/28, got %s", patchData.Spec.ReleasableCIDRs[0].CIDR)
		}
	})

	t.Run("Failure and Reconciliation", func(t *testing.T) {
		mockNNC.Spec.ReleasableCIDRs = []nncv1.PodCIDR{} // Reset
		patchCount = 0
		patchErr = errors.New("failed to patch")

		err = c.dynamicAllocation(context.Background(), network)
		if err == nil {
			t.Fatal("Expected dynamicAllocation to fail")
		}

		if patchCount != 1 {
			t.Errorf("Expected patch to be attempted once, got %d", patchCount)
		}

		// Second call: should succeed and reconcile
		patchErr = nil
		patchCount = 0

		err = c.dynamicAllocation(context.Background(), network)
		if err != nil {
			t.Fatalf("dynamicAllocation failed on second attempt: %v", err)
		}

		if patchCount != 1 {
			t.Errorf("Expected patch to be called once on second attempt, got %d", patchCount)
		}

		var patchData struct {
			Spec nncv1.NodeNetworkConfigSpec `json:"spec"`
		}
		json.Unmarshal(patchedData, &patchData)

		if len(patchData.Spec.ReleasableCIDRs) != 1 {
			t.Errorf("Expected 1 releasable CIDR in patch on second attempt, got %d", len(patchData.Spec.ReleasableCIDRs))
		} else if patchData.Spec.ReleasableCIDRs[0].CIDR != "10.0.2.0/28" {
			t.Errorf("Expected releasable CIDR 10.0.2.0/28, got %s", patchData.Spec.ReleasableCIDRs[0].CIDR)
		}

		if len(patchData.Spec.Allocations) != 1 {
			t.Errorf("Expected 1 allocation in patch on second attempt, got %d", len(patchData.Spec.Allocations))
		} else if patchData.Spec.Allocations[0].Pods != 0 {
			t.Errorf("Expected target pods to be 0 after reconciliation, got %d", patchData.Spec.Allocations[0].Pods)
		}
	})
}
