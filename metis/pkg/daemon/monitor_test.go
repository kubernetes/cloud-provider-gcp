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
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/go-logr/logr"
	nncv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/nodenetworkconfig/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	adaptiveipam "k8s.io/metis/api/adaptiveipam/v1"
	"k8s.io/metis/pkg/store"
)

func TestMonitor_DynamicAllocation_HighUtilization(t *testing.T) {
	logger := logr.Discard()
	network := "test-network"
	nodeName := "test-node"

	tests := []struct {
		desc                    string
		blocks                  []struct {
			cidr  string
			drain bool
		}
		allocations             int
		cooldowns               int
		pendingRequests         int
		mockNNC                 *nncv1.NodeNetworkConfig
		injectPatchErr          error
		expectedPatchCalled     bool
		expectedPatchedPods     int32
		expectedQueueLen        int
		expectedErr             bool
	}{
		{
			desc: "High Utilization triggers scale up",
			blocks: []struct {
				cidr  string
				drain bool
			}{
				{"10.0.1.0/28", false},
				{"10.0.2.0/27", false},
			},
			allocations:     42,
			pendingRequests: 10,
			mockNNC: &nncv1.NodeNetworkConfig{
				ObjectMeta: metav1.ObjectMeta{Name: nodeName},
				Spec: nncv1.NodeNetworkConfigSpec{
					Allocations: []nncv1.Allocation{{Network: network, Pods: 32}},
				},
				Status: nncv1.NodeNetworkConfigStatus{
					PodCIDRs: []nncv1.PodCIDR{{CIDR: "10.0.2.0/27", Network: network}},
				},
			},
			expectedPatchCalled: true,
			expectedPatchedPods: 58,
		},
		{
			desc: "No-op when zero initial IPs",
			blocks:          nil,
			allocations:     0,
			pendingRequests: 10,
			mockNNC: &nncv1.NodeNetworkConfig{
				ObjectMeta: metav1.ObjectMeta{Name: nodeName},
				Spec: nncv1.NodeNetworkConfigSpec{
					Allocations: []nncv1.Allocation{{Network: network, Pods: 32}},
				},
				Status: nncv1.NodeNetworkConfigStatus{PodCIDRs: []nncv1.PodCIDR{}},
			},
			expectedPatchCalled: false,
		},
		{
			desc: "Exclude draining from exhaustion",
			blocks: []struct {
				cidr  string
				drain bool
			}{
				{"10.0.1.0/28", false},
				{"10.0.2.0/27", false},
				{"10.0.3.0/27", true},
			},
			allocations:     40,
			pendingRequests: 10,
			mockNNC: &nncv1.NodeNetworkConfig{
				ObjectMeta: metav1.ObjectMeta{Name: nodeName},
				Spec: nncv1.NodeNetworkConfigSpec{
					Allocations: []nncv1.Allocation{{Network: network, Pods: 64}},
				},
				Status: nncv1.NodeNetworkConfigStatus{
					PodCIDRs: []nncv1.PodCIDR{{CIDR: "10.0.2.0/27", Network: network}},
				},
			},
			expectedPatchCalled: false,
		},
		{
			desc: "Cooldown pushback",
			blocks: []struct {
				cidr  string
				drain bool
			}{
				{"10.0.1.0/28", false},
				{"10.0.2.0/25", false},
			},
			allocations:     60,
			cooldowns:       11,
			pendingRequests: 10,
			mockNNC: &nncv1.NodeNetworkConfig{
				ObjectMeta: metav1.ObjectMeta{Name: nodeName},
				Spec: nncv1.NodeNetworkConfigSpec{
					Allocations: []nncv1.Allocation{{Network: network, Pods: 64}},
				},
				Status: nncv1.NodeNetworkConfigStatus{
					PodCIDRs: []nncv1.PodCIDR{{CIDR: "10.0.2.0/25", Network: network}},
				},
			},
			expectedPatchCalled: false,
			expectedQueueLen:    1,
		},
		{
			desc: "Patch failure returns error",
			blocks: []struct {
				cidr  string
				drain bool
			}{
				{"10.0.1.0/28", false},
			},
			allocations:     13,
			pendingRequests: 11,
			mockNNC: &nncv1.NodeNetworkConfig{
				ObjectMeta: metav1.ObjectMeta{Name: nodeName},
				Spec: nncv1.NodeNetworkConfigSpec{
					Allocations: []nncv1.Allocation{{Network: network, Pods: 16}},
				},
			},
			injectPatchErr:      fmt.Errorf("patch failed"),
			expectedPatchCalled: true,
			expectedErr:         true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			tempDir := t.TempDir()
			dbPath := filepath.Join(tempDir, "metis_monitor_high_util_test.sqlite")
			storeInstance, err := store.NewStore(context.Background(), logger, dbPath)
			if err != nil {
				t.Fatalf("Failed to create store: %v", err)
			}
			defer storeInstance.Close()

			for _, b := range tc.blocks {
				err = storeInstance.AddCIDR(context.Background(), network, b.cidr)
				if err != nil {
					t.Fatalf("Failed to add CIDR: %v", err)
				}
				if b.drain {
					id, exists, err := storeInstance.GetCIDRBlockByCIDR(context.Background(), b.cidr)
					if err != nil {
						t.Fatalf("GetCIDRBlockByCIDR failed: %v", err)
					}
					if !exists {
						t.Fatalf("Failed to find CIDR block for %s in store", b.cidr)
					}
					err = storeInstance.DrainCIDRBlock(context.Background(), id)
					if err != nil {
						t.Fatalf("DrainCIDRBlock failed: %v", err)
					}
				}
			}

			for i := 0; i < tc.cooldowns; i++ {
				containerID := fmt.Sprintf("cooldown-container-%d", i)
				_, _, err = storeInstance.AllocateIPv4(context.Background(), network, "eth0", containerID)
				if err != nil {
					t.Fatalf("Failed to allocate IP for cooldown: %v", err)
				}
				_, err = storeInstance.ReleaseIPByOwner(context.Background(), network, containerID, "eth0", 1*time.Hour)
				if err != nil {
					t.Fatalf("Failed to release IP for cooldown: %v", err)
				}
			}

			for i := 0; i < tc.allocations; i++ {
				_, _, err = storeInstance.AllocateIPv4(context.Background(), network, "eth0", fmt.Sprintf("container-%d", i))
				if err != nil {
					t.Fatalf("Failed to allocate IP: %v", err)
				}
			}

			patchCalled := false
			var patchedData []byte

			mockInterface := &mockNodeNetworkConfigInterface{
				getFunc: func(ctx context.Context, name string, opts metav1.GetOptions) (*nncv1.NodeNetworkConfig, error) {
					return tc.mockNNC, nil
				},
				patchFunc: func(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (*nncv1.NodeNetworkConfig, error) {
					patchCalled = true
					patchedData = data
					if tc.injectPatchErr != nil {
						return nil, tc.injectPatchErr
					}
					return tc.mockNNC, nil
				},
			}

			mockNetV1 := &mockNetworkingV1{nncInterface: mockInterface}
			mockClient := &mockClientset{networkingV1: mockNetV1}

			m := NewMonitor(MonitorConfig{
				Logger:                  logger,
				NNCClient:               mockClient,
				Store:                   storeInstance,
				NodeName:                nodeName,
				GetPendingRequestsCount: func(net string) int { return tc.pendingRequests },
				CooldownPushbackInterval: 1 * time.Millisecond,
			})

			err = m.syncNetwork(context.Background(), network)
			if tc.expectedErr {
				if err == nil {
					t.Errorf("Expected syncNetwork to fail, but it succeeded")
				}
				return
			}
			if err != nil {
				t.Fatalf("syncNetwork failed: %v", err)
			}

			if patchCalled != tc.expectedPatchCalled {
				t.Errorf("Expected patchCalled %v, got %v", tc.expectedPatchCalled, patchCalled)
			}

			if tc.expectedPatchCalled && patchedData != nil {
				var patch struct {
					Spec nncv1.NodeNetworkConfigSpec `json:"spec"`
				}
				err = json.Unmarshal(patchedData, &patch)
				if err != nil {
					t.Fatalf("Failed to unmarshal patch data: %v", err)
				}
				if len(patch.Spec.Allocations) == 0 {
					t.Fatal("Expected allocations to be non-empty in patch")
				}
				if patch.Spec.Allocations[0].Pods != tc.expectedPatchedPods {
					t.Errorf("Expected new target pods to be %d, got %d", tc.expectedPatchedPods, patch.Spec.Allocations[0].Pods)
				}
			}

			if tc.expectedQueueLen > 0 {
				time.Sleep(10 * time.Millisecond)
			}

			if m.queue.Len() != tc.expectedQueueLen {
				t.Errorf("Expected queue length %d, got %d", tc.expectedQueueLen, m.queue.Len())
			}
		})
	}
}

func TestMonitor_DynamicAllocation_LowUtilization(t *testing.T) {
	logger := logr.Discard()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "metis_monitor_low_util_test.sqlite")
	storeInstance, err := store.NewStore(context.Background(), logger, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer storeInstance.Close()

	network := "test-network"
	nodeName := "test-node"

	err = storeInstance.AddCIDR(context.Background(), network, "10.0.1.0/28") // 16 IPs
	if err != nil {
		t.Fatalf("Failed to add CIDR: %v", err)
	}

	err = storeInstance.AddCIDR(context.Background(), network, "10.0.2.0/27") // 32 IPs
	if err != nil {
		t.Fatalf("Failed to add CIDR: %v", err)
	}

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

	m := NewMonitor(MonitorConfig{
		Logger:    logger,
		NNCClient: mockClient,
		Store:     storeInstance,
		NodeName:  nodeName,
	})

	m.lowUtilizationTimers[network] = time.Now().Add(-9 * time.Hour)

	err = m.syncNetwork(context.Background(), network)
	if err != nil {
		t.Fatalf("syncNetwork failed: %v", err)
	}

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

func TestMonitor_MaybeDrainExcessive(t *testing.T) {
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
			utilization: DefaultLowUtilizationThreshold + 0.1,
			expectedTimerExists: false,
		},
		{
			desc: "Utilization above threshold no-op when no timer",
			setTimer: false,
			utilization: DefaultLowUtilizationThreshold + 0.1,
			expectedTimerExists: false,
		},
		{
			desc: "Utilization below threshold starts timer",
			setTimer: false,
			utilization: DefaultLowUtilizationThreshold - 0.1,
			expectedTimerExists: true,
		},
		{
			desc: "Utilization below threshold maintains timer",
			setTimer: true,
			timerDuration: -1 * time.Hour,
			utilization: DefaultLowUtilizationThreshold - 0.1,
			expectedTimerExists: true,
			expectedTimerUnchanged: true,
		},
		{
			desc: "Utilization below threshold for sustained duration triggers drain",
			setTimer: true,
			timerDuration: -9 * time.Hour,
			utilization: DefaultLowUtilizationThreshold - 0.1,
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
				dbPath := filepath.Join(tempDir, "metis_monitor_table_test.sqlite")
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

			m := NewMonitor(MonitorConfig{
				Logger:   logger,
				Store:    storeInstance,
				NodeName: nodeName,
			})

			var startTime time.Time
			if tc.setTimer {
				startTime = time.Now().Add(tc.timerDuration)
				m.lowUtilizationTimers[network] = startTime
			}

			info := &UtilizationInfo{
				Utilization: tc.utilization,
				InitialIPs:  tc.initialIPs,
				TargetPods:  tc.targetPods,
				Usage:       tc.usage,
			}

			drained := m.maybeDrainExcessive(ctx, network, info)

			if drained != tc.expectedDrained {
				t.Errorf("Expected drained %v, got %v", tc.expectedDrained, drained)
			}

			timer, ok := m.lowUtilizationTimers[network]
			if ok != tc.expectedTimerExists {
				t.Errorf("Expected timer exists %v, got %v", tc.expectedTimerExists, ok)
			}

			if ok && tc.expectedTimerUnchanged && !timer.Equal(startTime) {
				t.Errorf("Expected timer to be unchanged, but it was modified")
			}
		})
	}
}

func TestMonitor_HandleExpiredDrainingBlocks(t *testing.T) {
	logger := klog.Background()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "metis_monitor_expired_test.sqlite")
	storeInstance, err := store.NewStore(context.Background(), logger, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer storeInstance.Close()

	network := "test-network"
	nodeName := "test-node"

	err = storeInstance.AddCIDR(context.Background(), network, "10.0.1.0/28")
	if err != nil {
		t.Fatalf("Failed to add CIDR: %v", err)
	}

	err = storeInstance.AddCIDR(context.Background(), network, "10.0.2.0/28")
	if err != nil {
		t.Fatalf("Failed to add CIDR: %v", err)
	}

	readyBlocks, err := storeInstance.GetReadyCIDRBlocksSorted(context.Background(), network)
	if err != nil || len(readyBlocks) < 2 {
		t.Fatalf("Failed to get ready blocks: %v", err)
	}

	err = storeInstance.DrainCIDRBlock(context.Background(), readyBlocks[0].ID)
	if err != nil {
		t.Fatalf("DrainCIDRBlock failed: %v", err)
	}

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

	m := NewMonitor(MonitorConfig{
		Logger:             logger,
		NNCClient:          mockClient,
		Store:              storeInstance,
		NodeName:           nodeName,
		DrainingExpiration: 1 * time.Second,
	})

	t.Run("Success scenario adds to ReleasableCIDRs", func(t *testing.T) {
		patchCount = 0
		patchErr = nil
		mockNNC.Spec.ReleasableCIDRs = []nncv1.PodCIDR{}

		info, err := m.getUtilizationInfo(context.Background(), network)
		if err != nil {
			t.Fatalf("getUtilizationInfo failed: %v", err)
		}
		updated, err := m.handleExpiredDrainingBlocks(context.Background(), network, info.NncCopy, info.CurrentAllocation)
		if err != nil {
			t.Fatalf("handleExpiredDrainingBlocks failed: %v", err)
		}
		if updated {
			err = m.patchNNC(context.Background(), info)
			if err != nil {
				t.Fatalf("patchNNC failed: %v", err)
			}
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
}

func TestMonitor_ConcurrentAllocationAndDraining(t *testing.T) {
	logger := logr.Discard()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "metis_monitor_concurrent_test.sqlite")
	storeInstance, err := store.NewStore(context.Background(), logger, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer storeInstance.Close()

	network := "test-network"
	nodeName := "test-node"

	err = storeInstance.AddCIDR(context.Background(), network, "10.0.1.0/28")
	if err != nil {
		t.Fatalf("Failed to add CIDR: %v", err)
	}
	err = storeInstance.AddCIDR(context.Background(), network, "10.0.2.0/27")
	if err != nil {
		t.Fatalf("Failed to add CIDR: %v", err)
	}

	server := newAdaptiveIpamServer(logger, storeInstance, "", 0, 0)
	m := NewMonitor(MonitorConfig{
		Logger:   logger,
		Store:    storeInstance,
		NodeName: nodeName,
	})
	server.monitor = m

	m.lowUtilizationTimers[network] = time.Now().Add(-9 * time.Hour)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				m.syncNetwork(ctx, network)
				time.Sleep(10 * time.Millisecond)
			}
		}
	}()

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

	readyBlocks, err := storeInstance.GetReadyCIDRBlocksSorted(context.Background(), network)
	if err != nil {
		t.Fatalf("GetReadyCIDRBlocksSorted failed: %v", err)
	}
	
	if len(readyBlocks) == 0 {
		t.Errorf("Expected at least the initial block to be Ready")
	}
}

func TestMonitor_FullLoop(t *testing.T) {
	logger := logr.Discard()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "metis_monitor_full_loop_test.sqlite")
	storeInstance, err := store.NewStore(context.Background(), logger, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer storeInstance.Close()

	network := "test-network"
	nodeName := "test-node"

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

	m := NewMonitor(MonitorConfig{
		Logger:          logger,
		NNCClient:       mockClient,
		Store:           storeInstance,
		NodeName:        nodeName,
		MonitorInterval: 100 * time.Millisecond,
	})
	server := newAdaptiveIpamServer(logger, storeInstance, "", 0, 0)
	server.monitor = m

	// Create Watcher to simulate CIDR sync
	w := NewWatcher(logger, mockClient, nil, storeInstance, nodeName, server.onCIDRAdded)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	go m.Run(ctx, 1)

	m.GetPendingRequestsCount = func(net string) int {
		return 15
	}

	m.Enqueue(network)

	// Wait for Monitor to process and patch NNC
	time.Sleep(500 * time.Millisecond)

	// Now call syncCIDR manually to update DB!
	err = w.syncCIDR(ctx, network)
	if err != nil {
		t.Fatalf("syncCIDR failed: %v", err)
	}

	_, exists, err := storeInstance.GetCIDRBlockByCIDR(context.Background(), "10.0.2.0/28")
	if err != nil {
		t.Fatalf("GetCIDRBlockByCIDR failed: %v", err)
	}
	if !exists {
		t.Errorf("Expected new CIDR block 10.0.2.0/28 to exist in DB")
	}
}
