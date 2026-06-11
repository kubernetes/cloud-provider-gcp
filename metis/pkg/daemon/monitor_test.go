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

	nncv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/nodenetworkconfig/v1"
	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"k8s.io/metis/pkg/store"
)

func TestMonitor_DynamicAllocation_ScaleUp(t *testing.T) {
	logger := logr.Discard()
	network := "test-network"
	nodeName := "test-node"

	tests := []struct {
		desc   string
		blocks []struct {
			cidr  string
			drain bool
		}
		allocations         int
		cooldowns           int
		pendingRequests     int
		mockNNC             *nncv1.NodeNetworkConfig
		injectPatchErr      error
		expectedPatchCalled bool
		expectedPatchedPods int32
		expectedQueueLen    int
		expectedErr         bool
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
					PodCIDRs: []nncv1.PodCIDR{
						{CIDR: "10.0.1.0/28", Network: network},
						{CIDR: "10.0.2.0/27", Network: network},
					},
				},
			},
			expectedPatchCalled: true,
			// Expected pods calculation:
			// used = 42 (allocations) + 3 (reserved) = 45.
			// pending = 10, total = 48.
			// desired = ceil((45+10)/0.75) = ceil(73.33) = 74.
			// min = 48 + 10 = 58.
			// max(74, 58) = 74.
			expectedPatchedPods: 74,
		},
		{
			desc:            "No-op when zero initial IPs",
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
					PodCIDRs: []nncv1.PodCIDR{
						{CIDR: "10.0.1.0/28", Network: network},
						{CIDR: "10.0.2.0/27", Network: network},
						{CIDR: "10.0.3.0/27", Network: network},
					},
				},
			},
			expectedPatchCalled: true,
			// Expected pods calculation:
			// used = 40 (allocations) + 3 (reserved) = 43.
			// pending = 10, total = 80 (includes draining block).
			// desired = ceil((43+10)/0.75) = ceil(70.66) = 71.
			// min = 80 + 10 = 90.
			// max(71, 90) = 90.
			expectedPatchedPods: 90,
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
					PodCIDRs: []nncv1.PodCIDR{
						{CIDR: "10.0.1.0/28", Network: network},
						{CIDR: "10.0.2.0/25", Network: network},
					},
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
				Status: nncv1.NodeNetworkConfigStatus{
					PodCIDRs: []nncv1.PodCIDR{
						{CIDR: "10.0.1.0/28", Network: network},
					},
				},
			},
			injectPatchErr:      fmt.Errorf("patch failed"),
			expectedPatchCalled: true,
			expectedErr:         true,
		},
		{
			desc: "No scale up needed because current allocation is already equal to desired",
			blocks: []struct {
				cidr  string
				drain bool
			}{
				{"10.0.1.0/28", false},
				{"10.0.2.0/27", false},
			},
			allocations:     10,
			pendingRequests: 0,
			mockNNC: &nncv1.NodeNetworkConfig{
				ObjectMeta: metav1.ObjectMeta{Name: nodeName},
				Spec: nncv1.NodeNetworkConfigSpec{
					Allocations: []nncv1.Allocation{{Network: network, Pods: 48}},
				},
				Status: nncv1.NodeNetworkConfigStatus{
					PodCIDRs: []nncv1.PodCIDR{
						{CIDR: "10.0.1.0/28", Network: network},
						{CIDR: "10.0.2.0/27", Network: network},
					},
				},
			},
			expectedPatchCalled: false,
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
					id, exists, err := storeInstance.GetCIDRBlockByCIDRAndNetwork(context.Background(), b.cidr, network)
					if err != nil {
						t.Fatalf("GetCIDRBlockByCIDRAndNetwork failed: %v", err)
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
				_, _, err = storeInstance.AllocateIP(context.Background(), store.AllocateIPParams{Network: network, InterfaceName: "eth0", ContainerID: containerID, IPFamily: store.IPv4})
				if err != nil {
					t.Fatalf("Failed to allocate IP for cooldown: %v", err)
				}
				_, err = storeInstance.ReleaseIPByOwner(context.Background(), network, containerID, "eth0", 1*time.Hour)
				if err != nil {
					t.Fatalf("Failed to release IP for cooldown: %v", err)
				}
			}

			for i := 0; i < tc.allocations; i++ {
				_, _, err = storeInstance.AllocateIP(context.Background(), store.AllocateIPParams{Network: network, InterfaceName: "eth0", ContainerID: fmt.Sprintf("container-%d", i), IPFamily: store.IPv4})
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
				Logger:                   logger,
				NNCClient:                mockClient,
				Store:                    storeInstance,
				NodeName:                 nodeName,
				GetPendingRequestsCount:  func(net string) int { return tc.pendingRequests },
				CooldownPushbackInterval: 1 * time.Millisecond,
			})

			err = m.syncAll(context.Background())
			if tc.expectedErr {
				if err == nil {
					t.Errorf("Expected syncAll to fail, but it succeeded")
				}
				return
			}
			if err != nil {
				t.Fatalf("syncAll failed: %v", err)
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

func TestMonitor_DynamicAllocation_drainExcessive(t *testing.T) {
	logger := logr.Discard()
	network := "test-network"
	nodeName := "test-node"

	tests := []struct {
		desc   string
		blocks []struct {
			cidr  string
			drain bool
		}
		allocations         int
		cooldowns           int
		pendingRequests     int
		expectedReadyBlocks int
	}{
		{
			desc: "All excessive blocks drained when utilization is very low",
			blocks: []struct {
				cidr  string
				drain bool
			}{
				{"10.0.1.0/28", false}, // 16 IPs (initial)
				{"10.0.2.0/27", false}, // 32 IPs
				{"10.0.3.0/27", false}, // 32 IPs
			},
			allocations:         0,
			pendingRequests:     0,
			expectedReadyBlocks: 1, // Only initial block left
		},
		{
			desc: "Initial block is never drained",
			blocks: []struct {
				cidr  string
				drain bool
			}{
				{"10.0.1.0/28", false}, // 16 IPs (initial)
			},
			allocations:         0,
			pendingRequests:     0,
			expectedReadyBlocks: 1,
		},
		{
			desc: "Cooldowns are not considered as allocated",
			blocks: []struct {
				cidr  string
				drain bool
			}{
				{"10.0.1.0/28", false}, // 16 IPs (initial)
				{"10.0.2.0/27", false}, // 32 IPs
			},
			allocations:     2,  // 2 allocated IPs (+3 reserved = 5 used)
			cooldowns:       20, // 20 IPs in cooldown
			pendingRequests: 0,
			// Total = 48. Target = 24.
			// Used = 5 (ignores cooldown).
			// 5 < 24 -> Drains block 2!
			// Leaving only initial block ready.
			expectedReadyBlocks: 1,
		},
		{
			desc: "No-op when utilization is high enough",
			blocks: []struct {
				cidr  string
				drain bool
			}{
				{"10.0.1.0/28", false}, // 16 IPs (initial)
				{"10.0.2.0/27", false}, // 32 IPs
			},
			allocations:     25, // 25 allocated IPs (+3 reserved = 28 used)
			pendingRequests: 0,
			// Total = 48. Target = 24.
			// Used = 28 >= 24 -> No-op!
			expectedReadyBlocks: 2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			tempDir := t.TempDir()
			dbPath := filepath.Join(tempDir, "metis_monitor_low_util_test.sqlite")
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
					id, exists, err := storeInstance.GetCIDRBlockByCIDRAndNetwork(context.Background(), b.cidr, network)
					if err != nil {
						t.Fatalf("GetCIDRBlockByCIDRAndNetwork failed: %v", err)
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
				_, _, err = storeInstance.AllocateIP(context.Background(), store.AllocateIPParams{Network: network, InterfaceName: "eth0", ContainerID: containerID, IPFamily: store.IPv4})
				if err != nil {
					t.Fatalf("Failed to allocate IP for cooldown: %v", err)
				}
				_, err = storeInstance.ReleaseIPByOwner(context.Background(), network, containerID, "eth0", 1*time.Hour)
				if err != nil {
					t.Fatalf("Failed to release IP for cooldown: %v", err)
				}
			}

			for i := 0; i < tc.allocations; i++ {
				_, _, err = storeInstance.AllocateIP(context.Background(), store.AllocateIPParams{Network: network, InterfaceName: "eth0", ContainerID: fmt.Sprintf("container-%d", i), IPFamily: store.IPv4})
				if err != nil {
					t.Fatalf("Failed to allocate IP: %v", err)
				}
			}

			mockNNC := &nncv1.NodeNetworkConfig{
				ObjectMeta: metav1.ObjectMeta{Name: nodeName},
				Spec:       nncv1.NodeNetworkConfigSpec{},
			}

			mockInterface := &mockNodeNetworkConfigInterface{
				getFunc: func(ctx context.Context, name string, opts metav1.GetOptions) (*nncv1.NodeNetworkConfig, error) {
					return mockNNC, nil
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
			})

			m.lowUtilizationTimers[network] = time.Now().Add(-9 * time.Hour)

			err = m.syncAll(context.Background())
			if err != nil {
				t.Fatalf("syncAll failed: %v", err)
			}

			readyBlocks, err := storeInstance.GetReadyCIDRBlocksSorted(context.Background(), network)
			if err != nil {
				t.Fatalf("GetReadyCIDRBlocksSorted failed: %v", err)
			}

			if len(readyBlocks) != tc.expectedReadyBlocks {
				t.Errorf("Expected %d ready blocks, got %d", tc.expectedReadyBlocks, len(readyBlocks))
			}
		})
	}
}

func TestMonitor_MaybeDrainExcessive(t *testing.T) {
	logger := logr.Discard()
	network := "test-network"
	nodeName := "test-node"
	ctx := context.Background()

	tests := []struct {
		desc                   string
		setTimer               bool
		timerDuration          time.Duration
		utilization            float64
		targetPods             int
		usage                  store.NetworkIPUsage
		blocksToAdd            []string
		expectedTimerExists    bool
		expectedTimerUnchanged bool
		expectedDrained        bool
	}{
		{
			desc:                "Utilization above threshold resets timer",
			setTimer:            true,
			timerDuration:       0,
			utilization:         DefaultLowUtilizationThreshold + 0.1,
			expectedTimerExists: false,
		},
		{
			desc:                "Utilization above threshold no-op when no timer",
			setTimer:            false,
			utilization:         DefaultLowUtilizationThreshold + 0.1,
			expectedTimerExists: false,
		},
		{
			desc:                "Utilization below threshold starts timer",
			setTimer:            false,
			utilization:         DefaultLowUtilizationThreshold - 0.1,
			expectedTimerExists: true,
		},
		{
			desc:                   "Utilization below threshold maintains timer",
			setTimer:               true,
			timerDuration:          -1 * time.Hour,
			utilization:            DefaultLowUtilizationThreshold - 0.1,
			expectedTimerExists:    true,
			expectedTimerUnchanged: true,
		},
		{
			desc:                "Utilization below threshold for sustained duration triggers drain",
			setTimer:            true,
			timerDuration:       -9 * time.Hour,
			utilization:         DefaultLowUtilizationThreshold - 0.1,
			targetPods:          16,
			usage:               store.NetworkIPUsage{Total: 32},
			blocksToAdd:         []string{"10.0.1.0/28", "10.0.2.0/28"},
			expectedTimerExists: false,
			expectedDrained:     true,
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

func TestMonitor_syncDeletingBlocks(t *testing.T) {
	logger := logr.Discard()
	network := "test-network"
	nodeName := "test-node"

	tests := []struct {
		desc                    string
		setup                   func(t *testing.T, ctx context.Context, s *store.Store)
		initialNNC              *nncv1.NodeNetworkConfig
		expectedReleasableCIDRs []string
		expectedPods            *int32
		expectedPatchCount      int
	}{
		{
			desc: "Draining block expires and is added to ReleasableCIDRs",
			setup: func(t *testing.T, ctx context.Context, s *store.Store) {
				s.AddCIDR(ctx, network, "10.0.0.0/28") // Dummy initial block
				s.AddCIDR(ctx, network, "10.0.1.0/28")
				id, _, _ := s.GetCIDRBlockByCIDRAndNetwork(ctx, "10.0.1.0/28", network)
				s.DrainCIDRBlock(ctx, id)
				time.Sleep(1100 * time.Millisecond) // Wait for expiration (1s)
			},
			initialNNC: &nncv1.NodeNetworkConfig{
				ObjectMeta: metav1.ObjectMeta{Name: nodeName},
				Spec: nncv1.NodeNetworkConfigSpec{
					Allocations: []nncv1.Allocation{
						{Network: network, Pods: 32},
					},
				},
				Status: nncv1.NodeNetworkConfigStatus{
					PodCIDRs: []nncv1.PodCIDR{{CIDR: "10.0.1.0/28", Network: network}},
				},
			},
			expectedReleasableCIDRs: []string{"10.0.1.0/28"},
			expectedPods:            ptrInt32(16), // 32 - 16 = 16
			expectedPatchCount:      1,
		},
		{
			desc: "Deleting block in store but not in CRD is reconciled",
			setup: func(t *testing.T, ctx context.Context, s *store.Store) {
				s.AddCIDR(ctx, network, "10.0.0.0/28") // Dummy initial block
				s.AddCIDR(ctx, network, "10.0.2.0/28")
				id, _, _ := s.GetCIDRBlockByCIDRAndNetwork(ctx, "10.0.2.0/28", network)
				s.MarkCIDRBlockAsDeletingForTest(ctx, id) // Mark as deleting directly
				time.Sleep(100 * time.Millisecond)        // Give DB a moment
			},
			initialNNC: &nncv1.NodeNetworkConfig{
				ObjectMeta: metav1.ObjectMeta{Name: nodeName},
				Spec: nncv1.NodeNetworkConfigSpec{
					Allocations: []nncv1.Allocation{
						{Network: network, Pods: 32},
					},
				},
				Status: nncv1.NodeNetworkConfigStatus{
					PodCIDRs: []nncv1.PodCIDR{{CIDR: "10.0.2.0/28", Network: network}},
				},
			},
			expectedReleasableCIDRs: []string{"10.0.2.0/28"},
			expectedPods:            ptrInt32(16), // 32 - 16 = 16
			expectedPatchCount:      1,
		},
		{
			desc: "Both expired draining and missed deleting blocks are handled",
			setup: func(t *testing.T, ctx context.Context, s *store.Store) {
				// Expired draining
				s.AddCIDR(ctx, network, "10.0.2.0/28") // Dummy initial block
				s.AddCIDR(ctx, network, "10.0.3.0/28")
				id3, _, _ := s.GetCIDRBlockByCIDRAndNetwork(ctx, "10.0.3.0/28", network)
				s.DrainCIDRBlock(ctx, id3)

				// Missed deleting
				s.AddCIDR(ctx, network, "10.0.4.0/28")
				id4, _, _ := s.GetCIDRBlockByCIDRAndNetwork(ctx, "10.0.4.0/28", network)
				s.MarkCIDRBlockAsDeletingForTest(ctx, id4)

				time.Sleep(1100 * time.Millisecond) // Wait for expiration
			},
			initialNNC: &nncv1.NodeNetworkConfig{
				ObjectMeta: metav1.ObjectMeta{Name: nodeName},
				Spec: nncv1.NodeNetworkConfigSpec{
					Allocations: []nncv1.Allocation{
						{Network: network, Pods: 48},
					},
				},
				Status: nncv1.NodeNetworkConfigStatus{
					PodCIDRs: []nncv1.PodCIDR{
						{CIDR: "10.0.3.0/28", Network: network},
						{CIDR: "10.0.4.0/28", Network: network},
					},
				},
			},
			expectedReleasableCIDRs: []string{"10.0.3.0/28", "10.0.4.0/28"},
			expectedPods:            ptrInt32(16), // 48 - 16 - 16 = 16
			expectedPatchCount:      1,
		},
		{
			desc: "No-op when draining block is not expired",
			setup: func(t *testing.T, ctx context.Context, s *store.Store) {
				s.AddCIDR(ctx, network, "10.0.0.0/28") // Dummy initial block
				s.AddCIDR(ctx, network, "10.0.5.0/28")
				id, _, _ := s.GetCIDRBlockByCIDRAndNetwork(ctx, "10.0.5.0/28", network)
				s.DrainCIDRBlock(ctx, id)
				// Do not sleep, so it is not expired (expiration is 1s)
			},
			initialNNC: &nncv1.NodeNetworkConfig{
				ObjectMeta: metav1.ObjectMeta{Name: nodeName},
				Spec: nncv1.NodeNetworkConfigSpec{
					Allocations: []nncv1.Allocation{
						{Network: network, Pods: 32},
					},
				},
				Status: nncv1.NodeNetworkConfigStatus{
					PodCIDRs: []nncv1.PodCIDR{{CIDR: "10.0.5.0/28", Network: network}},
				},
			},
			expectedReleasableCIDRs: []string{},
			expectedPods:            ptrInt32(32),
			expectedPatchCount:      0,
		},
		{
			desc: "Deleting block already in ReleasableCIDRs does not reduce pods again",
			setup: func(t *testing.T, ctx context.Context, s *store.Store) {
				s.AddCIDR(ctx, network, "10.0.0.0/28") // Dummy initial block
				s.AddCIDR(ctx, network, "10.0.6.0/28")
				id, _, _ := s.GetCIDRBlockByCIDRAndNetwork(ctx, "10.0.6.0/28", network)
				s.MarkCIDRBlockAsDeletingForTest(ctx, id)
				time.Sleep(100 * time.Millisecond)
			},
			initialNNC: &nncv1.NodeNetworkConfig{
				ObjectMeta: metav1.ObjectMeta{Name: nodeName},
				Spec: nncv1.NodeNetworkConfigSpec{
					Allocations: []nncv1.Allocation{
						{Network: network, Pods: 16},
					},
					ReleasableCIDRs: []nncv1.PodCIDR{{CIDR: "10.0.6.0/28", Network: network}},
				},
				Status: nncv1.NodeNetworkConfigStatus{
					PodCIDRs: []nncv1.PodCIDR{{CIDR: "10.0.6.0/28", Network: network}},
				},
			},
			expectedReleasableCIDRs: []string{"10.0.6.0/28"},
			expectedPods:            ptrInt32(16),
			expectedPatchCount:      0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			tempDir := t.TempDir()
			dbPath := filepath.Join(tempDir, "metis_monitor_expired_test.sqlite")
			storeInstance, err := store.NewStore(context.Background(), logger, dbPath)
			if err != nil {
				t.Fatalf("Failed to create store: %v", err)
			}
			defer storeInstance.Close()

			tc.setup(t, context.Background(), storeInstance)

			var patchCount int
			var patchedData []byte
			var mu sync.Mutex

			mockInterface := &mockNodeNetworkConfigInterface{
				getFunc: func(ctx context.Context, name string, opts metav1.GetOptions) (*nncv1.NodeNetworkConfig, error) {
					return tc.initialNNC, nil
				},
				patchFunc: func(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (*nncv1.NodeNetworkConfig, error) {
					mu.Lock()
					defer mu.Unlock()
					patchCount++
					patchedData = data
					return tc.initialNNC, nil
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

			m.syncAll(context.Background())

			if patchCount != tc.expectedPatchCount {
				t.Errorf("Expected patch count %d, got %d", tc.expectedPatchCount, patchCount)
			}

			if tc.expectedPatchCount > 0 {
				var patch struct {
					Spec nncv1.NodeNetworkConfigSpec `json:"spec"`
				}
				json.Unmarshal(patchedData, &patch)

				if len(patch.Spec.ReleasableCIDRs) != len(tc.expectedReleasableCIDRs) {
					t.Errorf("Expected %d releasable CIDRs, got %d", len(tc.expectedReleasableCIDRs), len(patch.Spec.ReleasableCIDRs))
				}

				for _, expected := range tc.expectedReleasableCIDRs {
					found := false
					for _, r := range patch.Spec.ReleasableCIDRs {
						if r.CIDR == expected {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected CIDR %s to be in ReleasableCIDRs", expected)
					}
				}

				if tc.expectedPods != nil {
					foundAlloc := false
					for _, a := range patch.Spec.Allocations {
						if a.Network == network {
							foundAlloc = true
							if a.Pods != *tc.expectedPods {
								t.Errorf("Expected allocations pods to be %d, got %d", *tc.expectedPods, a.Pods)
							}
							break
						}
					}
					if !foundAlloc {
						t.Errorf("Expected to find allocation for network %s in patch", network)
					}
				}
			}
		})
	}
}

type monitorTestParams struct {
	name             string
	dbSetup          func(t *testing.T, storeInstance *store.Store, network string)
	initialNNC       func(nodeName, rv string, network string) *nncv1.NodeNetworkConfig
	getPendingCount  func(network string) int
	injectGetError   func(callCount int) error
	injectPatchError func(callCount int) error
	onGetCalled      func(callCount int, done func())
	onPatchCalled    func(callCount int, nnc *nncv1.NodeNetworkConfig, done func())
	verify           func(t *testing.T, getCount, patchCount int, patches [][]byte, expectedRV string)
}

func runMonitorTestHelper(t *testing.T, tc monitorTestParams) {
	logger := logr.Discard()
	network := "test-network"
	nodeName := "test-node"
	randomResourceVersion := fmt.Sprintf("rv-%d", time.Now().UnixNano())

	// 1. Setup DB
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, fmt.Sprintf("metis_monitor_test_%s.sqlite", filepath.Base(t.Name())))
	storeInstance, err := store.NewStore(context.Background(), logger, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer storeInstance.Close()

	if tc.dbSetup != nil {
		tc.dbSetup(t, storeInstance, network)
	}

	// 2. Setup mock objects & clients
	mockNNC := tc.initialNNC(nodeName, randomResourceVersion, network)

	var getCount, patchCount int
	var capturedPatches [][]byte
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(1)
	var once sync.Once
	done := func() {
		once.Do(func() {
			wg.Done()
		})
	}

	mockInterface := &mockNodeNetworkConfigInterface{
		getFunc: func(ctx context.Context, name string, opts metav1.GetOptions) (*nncv1.NodeNetworkConfig, error) {
			mu.Lock()
			getCount++
			count := getCount
			mu.Unlock()

			if tc.injectGetError != nil {
				if err := tc.injectGetError(count); err != nil {
					return nil, err
				}
			}

			if tc.onGetCalled != nil {
				tc.onGetCalled(count, done)
			}

			mu.Lock()
			defer mu.Unlock()
			return mockNNC, nil
		},
		patchFunc: func(ctx context.Context, name string, pt types.PatchType, data []byte, opts metav1.PatchOptions, subresources ...string) (*nncv1.NodeNetworkConfig, error) {
			mu.Lock()
			patchCount++
			count := patchCount
			mu.Unlock()

			if tc.injectPatchError != nil {
				if err := tc.injectPatchError(count); err != nil {
					return nil, err
				}
			}

			// Capture patch
			patchBytes := make([]byte, len(data))
			copy(patchBytes, data)
			mu.Lock()
			capturedPatches = append(capturedPatches, patchBytes)
			mu.Unlock()

			// Update mockNNC spec to simulate api server update
			var patch struct {
				Spec nncv1.NodeNetworkConfigSpec `json:"spec"`
			}
			if err := json.Unmarshal(data, &patch); err == nil {
				mu.Lock()
				if patch.Spec.Allocations != nil {
					mockNNC.Spec.Allocations = patch.Spec.Allocations
				}
				if patch.Spec.ReleasableCIDRs != nil {
					mockNNC.Spec.ReleasableCIDRs = patch.Spec.ReleasableCIDRs
				}
				mu.Unlock()
			}

			if tc.onPatchCalled != nil {
				tc.onPatchCalled(count, mockNNC, done)
			}

			mu.Lock()
			defer mu.Unlock()
			return mockNNC, nil
		},
	}
	mockNetV1 := &mockNetworkingV1{nncInterface: mockInterface}
	mockClient := &mockClientset{networkingV1: mockNetV1}

	m := NewMonitor(MonitorConfig{
		Logger:                  logger,
		NNCClient:               mockClient,
		Store:                   storeInstance,
		NodeName:                nodeName,
		MonitorInterval:         10 * time.Millisecond,  // Fast interval for tests
		DrainingExpiration:      100 * time.Millisecond, // Fast draining for tests
		GetPendingRequestsCount: tc.getPendingCount,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	runFinished := make(chan struct{})
	go func() {
		m.Run(ctx)
		close(runFinished)
	}()

	m.enqueue()

	waitFinished := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitFinished)
	}()

	select {
	case <-waitFinished:
		// Success
	case <-ctx.Done():
		mu.Lock()
		gCount, pCount := getCount, patchCount
		mu.Unlock()
		t.Fatalf("Timed out waiting for Monitor to reach target state. GetCount: %d, PatchCount: %d", gCount, pCount)
	}

	cancel()
	<-runFinished

	if !m.queue.ShuttingDown() {
		t.Error("Expected workqueue to be shutting down or shut down")
	}

	mu.Lock()
	gCount, pCount := getCount, patchCount
	patches := capturedPatches
	mu.Unlock()

	if tc.verify != nil {
		tc.verify(t, gCount, pCount, patches, randomResourceVersion)
	}
}

func TestMonitorRun(t *testing.T) {
	tests := []monitorTestParams{
		{
			name: "Success (Happy Path)",
			dbSetup: func(t *testing.T, storeInstance *store.Store, network string) {
				// Initial block (16 IPs)
				err := storeInstance.AddCIDR(context.Background(), network, "10.0.1.0/28")
				if err != nil {
					t.Fatalf("Failed to add CIDR: %v", err)
				}
				for i := 0; i < 10; i++ {
					_, _, err = storeInstance.AllocateIP(context.Background(), store.AllocateIPParams{Network: network, InterfaceName: "eth0", ContainerID: fmt.Sprintf("container-%d", i), IPFamily: store.IPv4})
					if err != nil {
						t.Fatalf("Failed to allocate IP: %v", err)
					}
				}

				// Deleting block (16 IPs)
				err = storeInstance.AddCIDR(context.Background(), network, "10.0.2.0/28")
				if err != nil {
					t.Fatalf("Failed to add CIDR: %v", err)
				}
				id, _, _ := storeInstance.GetCIDRBlockByCIDRAndNetwork(context.Background(), "10.0.2.0/28", network)
				err = storeInstance.MarkCIDRBlockAsDeletingForTest(context.Background(), id)
				if err != nil {
					t.Fatalf("Failed to mark CIDR as deleting: %v", err)
				}
			},
			initialNNC: func(nodeName, rv string, network string) *nncv1.NodeNetworkConfig {
				return &nncv1.NodeNetworkConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:            nodeName,
						ResourceVersion: rv,
					},
					Spec: nncv1.NodeNetworkConfigSpec{
						Allocations: []nncv1.Allocation{{Network: network, Pods: 16}},
					},
					Status: nncv1.NodeNetworkConfigStatus{
						PodCIDRs: []nncv1.PodCIDR{
							{CIDR: "10.0.1.0/28", Network: network},
							{CIDR: "10.0.2.0/28", Network: network},
						},
					},
				}
			},
			getPendingCount: func(net string) int { return 5 },
			onPatchCalled: func(callCount int, nnc *nncv1.NodeNetworkConfig, done func()) {
				if len(nnc.Spec.ReleasableCIDRs) == 1 &&
					nnc.Spec.ReleasableCIDRs[0].CIDR == "10.0.2.0/28" &&
					len(nnc.Spec.Allocations) == 1 &&
					nnc.Spec.Allocations[0].Pods == 24 {
					done()
				}
			},
			verify: func(t *testing.T, getCount, patchCount int, patches [][]byte, expectedRV string) {
				if len(patches) == 0 {
					t.Fatal("Expected at least one patch, got none")
				}
				lastPatchBytes := patches[len(patches)-1]
				var patch struct {
					Metadata struct {
						ResourceVersion string `json:"resourceVersion"`
					} `json:"metadata"`
					Spec nncv1.NodeNetworkConfigSpec `json:"spec"`
				}
				if err := json.Unmarshal(lastPatchBytes, &patch); err != nil {
					t.Fatalf("Failed to unmarshal final patch: %v", err)
				}

				if patch.Metadata.ResourceVersion != expectedRV {
					t.Errorf("Expected resourceVersion %q, got %q", expectedRV, patch.Metadata.ResourceVersion)
				}

				if len(patch.Spec.Allocations) != 1 || patch.Spec.Allocations[0].Pods != 24 {
					t.Errorf("Expected allocations of 24 pods, got %+v", patch.Spec.Allocations)
				}

				if len(patch.Spec.ReleasableCIDRs) != 1 || patch.Spec.ReleasableCIDRs[0].CIDR != "10.0.2.0/28" {
					t.Errorf("Expected releasable CIDR '10.0.2.0/28', got %+v", patch.Spec.ReleasableCIDRs)
				}
			},
		},
		{
			name: "Transient Get Failure",
			initialNNC: func(nodeName, rv string, network string) *nncv1.NodeNetworkConfig {
				return &nncv1.NodeNetworkConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:            nodeName,
						ResourceVersion: rv,
					},
				}
			},
			injectGetError: func(callCount int) error {
				if callCount == 1 {
					return fmt.Errorf("temporary get error")
				}
				return nil
			},
			onGetCalled: func(callCount int, done func()) {
				if callCount == 2 {
					done()
				}
			},
			verify: func(t *testing.T, getCount, patchCount int, patches [][]byte, expectedRV string) {
				if getCount < 2 {
					t.Errorf("Expected at least 2 attempts due to retry, got %d", getCount)
				}
			},
		},
		{
			name: "Transient Patch Conflict Retry",
			dbSetup: func(t *testing.T, storeInstance *store.Store, network string) {
				// Initial block (16 IPs)
				err := storeInstance.AddCIDR(context.Background(), network, "10.0.1.0/28")
				if err != nil {
					t.Fatalf("Failed to add CIDR: %v", err)
				}
				for i := 0; i < 10; i++ {
					_, _, err = storeInstance.AllocateIP(context.Background(), store.AllocateIPParams{Network: network, InterfaceName: "eth0", ContainerID: fmt.Sprintf("container-%d", i), IPFamily: store.IPv4})
					if err != nil {
						t.Fatalf("Failed to allocate IP: %v", err)
					}
				}

				// Deleting block (16 IPs)
				err = storeInstance.AddCIDR(context.Background(), network, "10.0.2.0/28")
				if err != nil {
					t.Fatalf("Failed to add CIDR: %v", err)
				}
				id, _, _ := storeInstance.GetCIDRBlockByCIDRAndNetwork(context.Background(), "10.0.2.0/28", network)
				err = storeInstance.MarkCIDRBlockAsDeletingForTest(context.Background(), id)
				if err != nil {
					t.Fatalf("Failed to mark CIDR as deleting: %v", err)
				}
			},
			initialNNC: func(nodeName, rv string, network string) *nncv1.NodeNetworkConfig {
				return &nncv1.NodeNetworkConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:            nodeName,
						ResourceVersion: rv,
					},
					Spec: nncv1.NodeNetworkConfigSpec{
						Allocations: []nncv1.Allocation{{Network: network, Pods: 16}},
					},
					Status: nncv1.NodeNetworkConfigStatus{
						PodCIDRs: []nncv1.PodCIDR{
							{CIDR: "10.0.1.0/28", Network: network},
							{CIDR: "10.0.2.0/28", Network: network},
						},
					},
				}
			},
			getPendingCount: func(net string) int { return 5 },
			injectPatchError: func(callCount int) error {
				if callCount == 1 {
					return apierrors.NewConflict(schema.GroupResource{Group: "networking.gke.io", Resource: "nodenetworkconfigs"}, "test-node", fmt.Errorf("conflict"))
				}
				return nil
			},
			onPatchCalled: func(callCount int, nnc *nncv1.NodeNetworkConfig, done func()) {
				if len(nnc.Spec.ReleasableCIDRs) == 1 &&
					nnc.Spec.ReleasableCIDRs[0].CIDR == "10.0.2.0/28" &&
					len(nnc.Spec.Allocations) == 1 &&
					nnc.Spec.Allocations[0].Pods == 24 {
					done()
				}
			},
			verify: func(t *testing.T, getCount, patchCount int, patches [][]byte, expectedRV string) {
				if patchCount < 2 {
					t.Errorf("Expected at least 2 patch calls due to retry after conflict, got %d", patchCount)
				}
				if len(patches) == 0 {
					t.Fatal("Expected at least one patch, got none")
				}
				lastPatchBytes := patches[len(patches)-1]
				var patch struct {
					Metadata struct {
						ResourceVersion string `json:"resourceVersion"`
					} `json:"metadata"`
					Spec nncv1.NodeNetworkConfigSpec `json:"spec"`
				}
				if err := json.Unmarshal(lastPatchBytes, &patch); err != nil {
					t.Fatalf("Failed to unmarshal final patch: %v", err)
				}

				if patch.Metadata.ResourceVersion != expectedRV {
					t.Errorf("Expected resourceVersion %q in patch, got %q", expectedRV, patch.Metadata.ResourceVersion)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runMonitorTestHelper(t, tc)
		})
	}
}

func ptrInt32(v int32) *int32 {
	return &v
}
