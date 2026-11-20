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
	"k8s.io/client-go/util/workqueue"

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
			desc: "Cooldown IPs within threshold count toward scale-up watermark",
			blocks: []struct {
				cidr  string
				drain bool
			}{
				{"10.0.1.0/28", false},
			},
			allocations:     3,
			cooldowns:       10,
			pendingRequests: 0,
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
			expectedPatchCalled: true,
			// Expected pods calculation:
			// allocated = 3 (allocations) + 3 (reserved) = 6 (< 12 watermark).
			// cooldown = 10 (<= 10 threshold).
			// used = 6 + 10 = 16.
			// desired = ceil(16/0.75) = ceil(21.33) = 22.
			expectedPatchedPods: 22,
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
					id, exists, err := storeInstance.GetCIDRBlock(context.Background(), b.cidr, network)
					if err != nil {
						t.Fatalf("GetCIDRBlock failed: %v", err)
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
				getFunc: func(_ context.Context, _ string, _ metav1.GetOptions) (*nncv1.NodeNetworkConfig, error) {
					return tc.mockNNC, nil
				},
				patchFunc: func(_ context.Context, _ string, _ types.PatchType, data []byte, _ metav1.PatchOptions, _ ...string) (*nncv1.NodeNetworkConfig, error) {
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
				GetPendingRequestsCount:  func(_ string) int { return tc.pendingRequests },
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
					id, exists, err := storeInstance.GetCIDRBlock(context.Background(), b.cidr, network)
					if err != nil {
						t.Fatalf("GetCIDRBlock failed: %v", err)
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
				getFunc: func(_ context.Context, _ string, _ metav1.GetOptions) (*nncv1.NodeNetworkConfig, error) {
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
				GetPendingRequestsCount: func(_ string) int { return tc.pendingRequests },
			})

			m.lowUtilizationTimers[network] = time.Now().Add(-9 * time.Hour)

			err = m.syncAll(context.Background())
			if err != nil {
				t.Fatalf("syncAll failed: %v", err)
			}

			readyBlocks, err := storeInstance.GetReadyCIDRBlocksSorted(context.Background(), network, store.IPv4)
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
			setup: func(_ *testing.T, ctx context.Context, s *store.Store) {
				s.AddCIDR(ctx, network, "10.0.0.0/28") // Dummy initial block
				s.AddCIDR(ctx, network, "10.0.1.0/28")
				id, _, _ := s.GetCIDRBlock(ctx, "10.0.1.0/28", network)
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
					PodCIDRs: []nncv1.PodCIDR{
						{CIDR: "10.0.0.0/28", Network: network},
						{CIDR: "10.0.1.0/28", Network: network},
					},
				},
			},
			expectedReleasableCIDRs: []string{"10.0.1.0/28"},
			expectedPods:            ptrInt32(16), // 32 - 16 = 16
			expectedPatchCount:      1,
		},
		{
			desc: "Deleting block in store but not in CRD is reconciled",
			setup: func(_ *testing.T, ctx context.Context, s *store.Store) {
				s.AddCIDR(ctx, network, "10.0.0.0/28") // Dummy initial block
				s.AddCIDR(ctx, network, "10.0.2.0/28")
				id, _, _ := s.GetCIDRBlock(ctx, "10.0.2.0/28", network)
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
					PodCIDRs: []nncv1.PodCIDR{
						{CIDR: "10.0.0.0/28", Network: network},
						{CIDR: "10.0.2.0/28", Network: network},
					},
				},
			},
			expectedReleasableCIDRs: []string{"10.0.2.0/28"},
			expectedPods:            ptrInt32(16), // 32 - 16 = 16
			expectedPatchCount:      1,
		},
		{
			desc: "Both expired draining and missed deleting blocks are handled",
			setup: func(_ *testing.T, ctx context.Context, s *store.Store) {
				// Expired draining
				s.AddCIDR(ctx, network, "10.0.2.0/28") // Dummy initial block
				s.AddCIDR(ctx, network, "10.0.3.0/28")
				id3, _, _ := s.GetCIDRBlock(ctx, "10.0.3.0/28", network)
				s.DrainCIDRBlock(ctx, id3)

				// Missed deleting
				s.AddCIDR(ctx, network, "10.0.4.0/28")
				id4, _, _ := s.GetCIDRBlock(ctx, "10.0.4.0/28", network)
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
						{CIDR: "10.0.2.0/28", Network: network},
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
			setup: func(_ *testing.T, ctx context.Context, s *store.Store) {
				s.AddCIDR(ctx, network, "10.0.0.0/28") // Dummy initial block
				s.AddCIDR(ctx, network, "10.0.5.0/28")
				id, _, _ := s.GetCIDRBlock(ctx, "10.0.5.0/28", network)
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
					PodCIDRs: []nncv1.PodCIDR{
						{CIDR: "10.0.0.0/28", Network: network},
						{CIDR: "10.0.5.0/28", Network: network},
					},
				},
			},
			expectedReleasableCIDRs: []string{},
			expectedPods:            ptrInt32(32),
			expectedPatchCount:      0,
		},
		{
			desc: "Deleting block already in ReleasableCIDRs does not reduce pods again",
			setup: func(_ *testing.T, ctx context.Context, s *store.Store) {
				s.AddCIDR(ctx, network, "10.0.0.0/28") // Dummy initial block
				s.AddCIDR(ctx, network, "10.0.6.0/28")
				id, _, _ := s.GetCIDRBlock(ctx, "10.0.6.0/28", network)
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
					PodCIDRs: []nncv1.PodCIDR{
						{CIDR: "10.0.0.0/28", Network: network},
						{CIDR: "10.0.6.0/28", Network: network},
					},
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
				getFunc: func(_ context.Context, _ string, _ metav1.GetOptions) (*nncv1.NodeNetworkConfig, error) {
					return tc.initialNNC, nil
				},
				patchFunc: func(_ context.Context, _ string, _ types.PatchType, data []byte, _ metav1.PatchOptions, _ ...string) (*nncv1.NodeNetworkConfig, error) {
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
	rateLimiter      workqueue.TypedRateLimiter[string]
	monitorInterval  time.Duration
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
		getFunc: func(_ context.Context, _ string, _ metav1.GetOptions) (*nncv1.NodeNetworkConfig, error) {
			mu.Lock()
			getCount++
			count := getCount
			mu.Unlock()

			if tc.onGetCalled != nil {
				tc.onGetCalled(count, done)
			}

			if tc.injectGetError != nil {
				if err := tc.injectGetError(count); err != nil {
					return nil, err
				}
			}

			mu.Lock()
			defer mu.Unlock()
			return mockNNC, nil
		},
		patchFunc: func(_ context.Context, _ string, _ types.PatchType, data []byte, _ metav1.PatchOptions, _ ...string) (*nncv1.NodeNetworkConfig, error) {
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

	interval := tc.monitorInterval
	if interval <= 0 {
		interval = 10 * time.Millisecond
	}

	m := NewMonitor(MonitorConfig{
		Logger:                  logger,
		NNCClient:               mockClient,
		Store:                   storeInstance,
		NodeName:                nodeName,
		MonitorInterval:         interval,
		DrainingExpiration:      100 * time.Millisecond, // Fast draining for tests
		GetPendingRequestsCount: tc.getPendingCount,
		RateLimiter:             tc.rateLimiter,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	runFinished := make(chan struct{})
	go func() {
		m.Run(ctx)
		close(runFinished)
	}()

	m.Enqueue()

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
				id, _, _ := storeInstance.GetCIDRBlock(context.Background(), "10.0.2.0/28", network)
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
			getPendingCount: func(_ string) int { return 5 },
			onPatchCalled: func(_ int, nnc *nncv1.NodeNetworkConfig, done func()) {
				if len(nnc.Spec.ReleasableCIDRs) == 1 &&
					nnc.Spec.ReleasableCIDRs[0].CIDR == "10.0.2.0/28" &&
					len(nnc.Spec.Allocations) == 1 &&
					nnc.Spec.Allocations[0].Pods == 24 {
					done()
				}
			},
			verify: func(t *testing.T, _, _ int, patches [][]byte, expectedRV string) {
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
			initialNNC: func(nodeName, rv string, _ string) *nncv1.NodeNetworkConfig {
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
			verify: func(t *testing.T, getCount, _ int, _ [][]byte, _ string) {
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
				id, _, _ := storeInstance.GetCIDRBlock(context.Background(), "10.0.2.0/28", network)
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
			getPendingCount: func(_ string) int { return 5 },
			injectPatchError: func(callCount int) error {
				if callCount == 1 {
					return apierrors.NewConflict(schema.GroupResource{Group: "networking.gke.io", Resource: "nodenetworkconfigs"}, "test-node", fmt.Errorf("conflict"))
				}
				return nil
			},
			onPatchCalled: func(_ int, nnc *nncv1.NodeNetworkConfig, done func()) {
				if len(nnc.Spec.ReleasableCIDRs) == 1 &&
					nnc.Spec.ReleasableCIDRs[0].CIDR == "10.0.2.0/28" &&
					len(nnc.Spec.Allocations) == 1 &&
					nnc.Spec.Allocations[0].Pods == 24 {
					done()
				}
			},
			verify: func(t *testing.T, _, patchCount int, patches [][]byte, expectedRV string) {
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
		{
			name: "Permanent Failure - Drop After 10 Retries",
			initialNNC: func(nodeName, rv string, _ string) *nncv1.NodeNetworkConfig {
				return &nncv1.NodeNetworkConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:            nodeName,
						ResourceVersion: rv,
					},
				}
			},
			injectGetError: func(_ int) error {
				return fmt.Errorf("permanent get error")
			},
			onGetCalled: func(callCount int, done func()) {
				if callCount == 11 {
					done()
				}
			},
			rateLimiter:     workqueue.NewTypedItemExponentialFailureRateLimiter[string](1*time.Millisecond, 10*time.Millisecond),
			monitorInterval: 1 * time.Hour,
			verify: func(t *testing.T, getCount, _ int, _ [][]byte, _ string) {
				if getCount != 11 {
					t.Errorf("Expected exactly 11 attempts (1 initial + 10 retries), got %d", getCount)
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

// TestMonitor_OutOfSyncCIDR_AllocatedZero covers a block that is deleted out of
// band in GCE while it has no pods on it: CCM has already dropped it from NNC
// status, and the next sync must delete it from SQLite on the spot. Capacity
// shrinks on the same tick, the block is not offered back through
// spec.releasableCIDRs, and the spec is not bumped because of it.
func TestMonitor_OutOfSyncCIDR_AllocatedZero(t *testing.T) {
	ctx := context.Background()
	logger := logr.Discard()
	network := "test-network"
	nodeName := "test-node"

	dbPath := filepath.Join(t.TempDir(), "test.sqlite")
	storeInstance, err := store.NewStore(ctx, logger, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer storeInstance.Close()

	// Add Block 1 (initial: has 3 reserved IPs) and Block 2 (0 allocated IPs)
	if err := storeInstance.AddCIDR(ctx, network, "10.0.1.0/28"); err != nil {
		t.Fatalf("Failed to add block 1: %v", err)
	}
	if err := storeInstance.AddCIDR(ctx, network, "10.0.2.0/28"); err != nil {
		t.Fatalf("Failed to add block 2: %v", err)
	}

	// Allocate 5 IPs from block 1
	for i := 0; i < 5; i++ {
		_, _, err := storeInstance.AllocateIP(ctx, store.AllocateIPParams{
			Network:       network,
			InterfaceName: "eth0",
			ContainerID:   fmt.Sprintf("container-b1-%d", i),
			IPFamily:      store.IPv4,
		})
		if err != nil {
			t.Fatalf("Failed to allocate IP: %v", err)
		}
	}

	// Block 2 has 0 allocated IPs.
	// Out-of-band deletion in GCE: Block 2 was deleted from GCE and CCM removed it from NNC.status.podCIDRs.
	// So NNC.status.podCIDRs only contains block 1 ("10.0.1.0/28").
	// NNC.spec.allocations currently has Pods: 32.
	nnc := &nncv1.NodeNetworkConfig{
		ObjectMeta: metav1.ObjectMeta{Name: nodeName, ResourceVersion: "1"},
		Spec: nncv1.NodeNetworkConfigSpec{
			Allocations: []nncv1.Allocation{
				{Network: network, Pods: 32},
			},
		},
		Status: nncv1.NodeNetworkConfigStatus{
			PodCIDRs: []nncv1.PodCIDR{
				{Network: network, CIDR: "10.0.1.0/28"},
			},
		},
	}

	patchCalled := false
	var patchedData []byte
	mockInterface := &mockNodeNetworkConfigInterface{
		getFunc: func(_ context.Context, _ string, _ metav1.GetOptions) (*nncv1.NodeNetworkConfig, error) {
			return nnc, nil
		},
		patchFunc: func(_ context.Context, _ string, _ types.PatchType, data []byte, _ metav1.PatchOptions, _ ...string) (*nncv1.NodeNetworkConfig, error) {
			patchCalled = true
			patchedData = data
			return nnc, nil
		},
	}
	mockClient := &mockClientset{networkingV1: &mockNetworkingV1{nncInterface: mockInterface}}

	m := NewMonitor(MonitorConfig{
		Logger:                  logger,
		NNCClient:               mockClient,
		Store:                   storeInstance,
		NodeName:                nodeName,
		GetPendingRequestsCount: func(_ string) int { return 0 },
	})

	if err := m.syncAll(ctx); err != nil {
		t.Fatalf("syncAll failed: %v", err)
	}

	// Block 2 is deleted from SQLite immediately.
	_, exists, err := storeInstance.GetCIDRBlock(ctx, "10.0.2.0/28", network)
	if err != nil {
		t.Fatalf("GetCIDRBlock failed: %v", err)
	}
	if exists {
		t.Errorf("Expected out-of-sync block 10.0.2.0/28 to be immediately deleted from DB, but it still exists")
	}

	// Block 1 is untouched.
	_, exists, err = storeInstance.GetCIDRBlock(ctx, "10.0.1.0/28", network)
	if err != nil || !exists {
		t.Fatalf("Expected block 10.0.1.0/28 to still exist in DB, exists: %v, err: %v", exists, err)
	}

	// Usage total reflects the cleanup on the same tick (16, not 32).
	usage, err := storeInstance.GetIPUsage(ctx, network, store.IPv4)
	if err != nil {
		t.Fatalf("GetIPUsage failed: %v", err)
	}
	if usage.Total != 16 {
		t.Errorf("Expected usage.Total to be 16 after deleting out-of-sync block, got %d", usage.Total)
	}

	// Block 2 was neither offered back via releasableCIDRs nor did it trigger a scale-up.
	if patchCalled {
		var patch struct {
			Spec nncv1.NodeNetworkConfigSpec `json:"spec"`
		}
		if err := json.Unmarshal(patchedData, &patch); err != nil {
			t.Fatalf("Failed to unmarshal patch: %v", err)
		}
		for _, r := range patch.Spec.ReleasableCIDRs {
			if r.CIDR == "10.0.2.0/28" {
				t.Errorf("Out-of-sync block should NOT be added to releasableCIDRs")
			}
		}
		if len(patch.Spec.Allocations) > 0 && patch.Spec.Allocations[0].Pods > 32 {
			t.Errorf("Expected no scale-up beyond currentPods 32, got %d", patch.Spec.Allocations[0].Pods)
		}
	}
}

// TestMonitor_OutOfSyncCIDR_AllocatedNonZero covers a block that is deleted out
// of band in GCE while pods still hold IPs from it. It cannot simply be
// dropped, so it is quarantined as Unknown: excluded from usage (so no scale-up
// loop), closed to new allocations, and kept until its last pod exits, at which
// point a later sync deletes it.
func TestMonitor_OutOfSyncCIDR_AllocatedNonZero(t *testing.T) {
	ctx := context.Background()
	logger := logr.Discard()
	network := "test-network"
	nodeName := "test-node"

	dbPath := filepath.Join(t.TempDir(), "test.sqlite")
	storeInstance, err := store.NewStore(ctx, logger, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer storeInstance.Close()

	if err := storeInstance.AddCIDR(ctx, network, "10.0.1.0/28"); err != nil {
		t.Fatalf("Failed to add block 1: %v", err)
	}
	if err := storeInstance.AddCIDR(ctx, network, "10.0.2.0/28"); err != nil {
		t.Fatalf("Failed to add block 2: %v", err)
	}

	// Allocate 13 IPs on block 1, so block 1 is full (3 reserved + 13 allocated = 16)
	for i := 0; i < 13; i++ {
		_, _, err := storeInstance.AllocateIP(ctx, store.AllocateIPParams{
			Network:       network,
			InterfaceName: "eth0",
			ContainerID:   fmt.Sprintf("container-b1-%d", i),
			IPFamily:      store.IPv4,
		})
		if err != nil {
			t.Fatalf("Failed to allocate IP on block 1: %v", err)
		}
	}

	// Allocate 2 IPs which go to block 2
	var b2Containers []string
	for i := 0; i < 2; i++ {
		cID := fmt.Sprintf("container-b2-%d", i)
		ip, cidr, err := storeInstance.AllocateIP(ctx, store.AllocateIPParams{
			Network:       network,
			InterfaceName: "eth0",
			ContainerID:   cID,
			IPFamily:      store.IPv4,
		})
		if err != nil {
			t.Fatalf("Failed to allocate IP on block 2: %v", err)
		}
		if cidr != "10.0.2.0/28" {
			t.Fatalf("Expected allocation from block 2, got %s for IP %s", cidr, ip)
		}
		b2Containers = append(b2Containers, cID)
	}

	// Block 2 is deleted out of band in GCE and removed from NNC status
	nnc := &nncv1.NodeNetworkConfig{
		ObjectMeta: metav1.ObjectMeta{Name: nodeName, ResourceVersion: "1"},
		Spec: nncv1.NodeNetworkConfigSpec{
			Allocations: []nncv1.Allocation{
				{Network: network, Pods: 32},
			},
		},
		Status: nncv1.NodeNetworkConfigStatus{
			PodCIDRs: []nncv1.PodCIDR{
				{Network: network, CIDR: "10.0.1.0/28"},
			},
		},
	}

	patchCalled := false
	var patchedData []byte
	mockInterface := &mockNodeNetworkConfigInterface{
		getFunc: func(_ context.Context, _ string, _ metav1.GetOptions) (*nncv1.NodeNetworkConfig, error) {
			return nnc, nil
		},
		patchFunc: func(_ context.Context, _ string, _ types.PatchType, data []byte, _ metav1.PatchOptions, _ ...string) (*nncv1.NodeNetworkConfig, error) {
			patchCalled = true
			patchedData = data
			return nnc, nil
		},
	}
	mockClient := &mockClientset{networkingV1: &mockNetworkingV1{nncInterface: mockInterface}}

	m := NewMonitor(MonitorConfig{
		Logger:                  logger,
		NNCClient:               mockClient,
		Store:                   storeInstance,
		NodeName:                nodeName,
		GetPendingRequestsCount: func(_ string) int { return 0 },
	})

	// First sync loop: Block 2 has 2 active pod allocations
	if err := m.syncAll(ctx); err != nil {
		t.Fatalf("syncAll failed: %v", err)
	}

	// Block 2 still has a pod, so it is marked Unknown rather than deleted.
	b2ID, exists, err := storeInstance.GetCIDRBlock(ctx, "10.0.2.0/28", network)
	if err != nil || !exists {
		t.Fatalf("Expected block 2 to still exist in DB, exists: %v, err: %v", exists, err)
	}
	blocks, err := storeInstance.GetNonDeletingCIDRBlocks(ctx, network, store.IPv4)
	if err != nil {
		t.Fatalf("GetNonDeletingCIDRBlocks failed: %v", err)
	}
	var b2State store.CidrBlockState
	for _, b := range blocks {
		if b.ID == b2ID {
			b2State = b.State
		}
	}
	if b2State != store.StateUnknown {
		t.Errorf("Expected block 2 state to be Unknown, got %q", b2State)
	}

	// GetIPUsage excludes the Unknown block from Total and Allocated right away.
	usage, err := storeInstance.GetIPUsage(ctx, network, store.IPv4)
	if err != nil {
		t.Fatalf("GetIPUsage failed: %v", err)
	}
	if usage.Total != 16 {
		t.Errorf("Expected usage.Total to be 16 (excluding Unknown block 2), got %d", usage.Total)
	}
	if usage.Allocated != 16 {
		t.Errorf("Expected usage.Allocated to be 16 (excluding Unknown block 2), got %d", usage.Allocated)
	}

	// No scale-up loop: spec pods must not have been bumped beyond 32.
	if patchCalled {
		var patch struct {
			Spec nncv1.NodeNetworkConfigSpec `json:"spec"`
		}
		if err := json.Unmarshal(patchedData, &patch); err != nil {
			t.Fatalf("Failed to unmarshal patch: %v", err)
		}
		if len(patch.Spec.Allocations) > 0 && patch.Spec.Allocations[0].Pods > 32 {
			t.Errorf("Scale-up loop detected: target pods was bumped to %d", patch.Spec.Allocations[0].Pods)
		}
	}

	// The pod on block 2 exits.
	for _, cID := range b2Containers {
		_, err := storeInstance.ReleaseIPByOwner(ctx, network, cID, "eth0", 0)
		if err != nil {
			t.Fatalf("Failed to release IP for %s: %v", cID, err)
		}
	}

	// Reset patchCalled
	patchCalled = false

	// Second sync loop: Now block 2 has 0 allocated IPs and is out of sync with status
	if err := m.syncAll(ctx); err != nil {
		t.Fatalf("syncAll second run failed: %v", err)
	}

	// With no pods left, block 2 is deleted from SQLite.
	_, exists, err = storeInstance.GetCIDRBlock(ctx, "10.0.2.0/28", network)
	if err != nil {
		t.Fatalf("GetCIDRBlock failed: %v", err)
	}
	if exists {
		t.Errorf("Expected block 2 to be deleted after active pods exited, but it still exists")
	}
}

// TestMonitor_OutOfSyncCIDR_RestoredToReady covers a false alarm: a block
// disappears from NNC status for one sync (e.g. a transient status rewrite) and
// comes back on the next. The block goes Ready -> Unknown -> Ready, counts
// towards usage again, and accepts new allocations again, without ever having
// been deleted.
func TestMonitor_OutOfSyncCIDR_RestoredToReady(t *testing.T) {
	ctx := context.Background()
	logger := logr.Discard()
	network := "test-network"
	nodeName := "test-node"

	dbPath := filepath.Join(t.TempDir(), "test.sqlite")
	storeInstance, err := store.NewStore(ctx, logger, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer storeInstance.Close()

	if err := storeInstance.AddCIDR(ctx, network, "10.0.1.0/28"); err != nil {
		t.Fatalf("Failed to add block 1: %v", err)
	}
	if err := storeInstance.AddCIDR(ctx, network, "10.0.2.0/28"); err != nil {
		t.Fatalf("Failed to add block 2: %v", err)
	}

	// Exhaust block 1 (13 allocations + 3 reserved = 16)
	for i := 0; i < 13; i++ {
		_, _, err := storeInstance.AllocateIP(ctx, store.AllocateIPParams{
			Network:       network,
			InterfaceName: "eth0",
			ContainerID:   fmt.Sprintf("container-b1-%d", i),
			IPFamily:      store.IPv4,
		})
		if err != nil {
			t.Fatalf("Failed to allocate IP on block 1: %v", err)
		}
	}

	// Allocate 1 IP on block 2
	_, cidr, err := storeInstance.AllocateIP(ctx, store.AllocateIPParams{
		Network:       network,
		InterfaceName: "eth0",
		ContainerID:   "container-b2-1",
		IPFamily:      store.IPv4,
	})
	if err != nil || cidr != "10.0.2.0/28" {
		t.Fatalf("Expected allocation on block 2, got cidr=%s, err=%v", cidr, err)
	}

	// CCM transiently omits block 2 from NNC.Status.PodCIDRs.
	nnc := &nncv1.NodeNetworkConfig{
		ObjectMeta: metav1.ObjectMeta{Name: nodeName, ResourceVersion: "1"},
		Spec: nncv1.NodeNetworkConfigSpec{
			Allocations: []nncv1.Allocation{
				{Network: network, Pods: 32},
			},
		},
		Status: nncv1.NodeNetworkConfigStatus{
			PodCIDRs: []nncv1.PodCIDR{
				{Network: network, CIDR: "10.0.1.0/28"},
			},
		},
	}

	mockInterface := &mockNodeNetworkConfigInterface{
		getFunc: func(_ context.Context, _ string, _ metav1.GetOptions) (*nncv1.NodeNetworkConfig, error) {
			return nnc, nil
		},
		patchFunc: func(_ context.Context, _ string, _ types.PatchType, _ []byte, _ metav1.PatchOptions, _ ...string) (*nncv1.NodeNetworkConfig, error) {
			return nnc, nil
		},
	}
	mockClient := &mockClientset{networkingV1: &mockNetworkingV1{nncInterface: mockInterface}}

	m := NewMonitor(MonitorConfig{
		Logger:                  logger,
		NNCClient:               mockClient,
		Store:                   storeInstance,
		NodeName:                nodeName,
		GetPendingRequestsCount: func(_ string) int { return 0 },
	})

	if err := m.syncAll(ctx); err != nil {
		t.Fatalf("First syncAll failed: %v", err)
	}

	// Verify block 2 is now Unknown
	b2ID, _, _ := storeInstance.GetCIDRBlock(ctx, "10.0.2.0/28", network)
	blocks, err := storeInstance.GetNonDeletingCIDRBlocks(ctx, network, store.IPv4)
	if err != nil {
		t.Fatalf("GetNonDeletingCIDRBlocks failed: %v", err)
	}
	for _, b := range blocks {
		if b.ID == b2ID && b.State != store.StateUnknown {
			t.Fatalf("Expected block 2 state to be Unknown, got %q", b.State)
		}
	}

	// CCM restores block 2 to NNC.Status.PodCIDRs.
	nnc.Status.PodCIDRs = append(nnc.Status.PodCIDRs, nncv1.PodCIDR{
		Network: network,
		CIDR:    "10.0.2.0/28",
	})

	if err := m.syncAll(ctx); err != nil {
		t.Fatalf("Second syncAll failed: %v", err)
	}

	// Verify block 2 is restored to Ready
	blocks, err = storeInstance.GetNonDeletingCIDRBlocks(ctx, network, store.IPv4)
	if err != nil {
		t.Fatalf("GetNonDeletingCIDRBlocks failed: %v", err)
	}
	for _, b := range blocks {
		if b.ID == b2ID && b.State != store.StateReady {
			t.Errorf("Expected block 2 state to be restored to Ready, got %q", b.State)
		}
	}

	// Verify usage includes block 2 again (Total = 32, Allocated = 17)
	usage, err := storeInstance.GetIPUsage(ctx, network, store.IPv4)
	if err != nil {
		t.Fatalf("GetIPUsage failed: %v", err)
	}
	if usage.Total != 32 || usage.Allocated != 17 {
		t.Errorf("Expected usage Total=32, Allocated=17 after restoring to Ready, got Total=%d, Allocated=%d", usage.Total, usage.Allocated)
	}

	// Verify new pods can allocate from block 2 again
	_, cidr, err = storeInstance.AllocateIP(ctx, store.AllocateIPParams{
		Network:       network,
		InterfaceName: "eth0",
		ContainerID:   "container-b2-2",
		IPFamily:      store.IPv4,
	})
	if err != nil || cidr != "10.0.2.0/28" {
		t.Errorf("Expected new allocation on restored block 2 to succeed, got cidr=%s, err=%v", cidr, err)
	}
}

// TestMonitor_OutOfSyncCIDR_NonReusable is the out-of-band deletion scenario
// for non-reusable /32 ANE blocks, where some of the deleted ANEs are empty and
// some still back a pod, and CCM has already published replacements. Empty ones
// are deleted immediately, occupied ones become Unknown, the replacements count
// towards capacity, and nothing is added to spec.releasableCIDRs or triggers a
// spec patch -- including when the occupied one is finally released by its pod
// (non-reusable blocks go straight to Deleting).
func TestMonitor_OutOfSyncCIDR_NonReusable(t *testing.T) {
	ctx := context.Background()
	logger := logr.Discard()
	network := "test-network"
	nodeName := "test-node"

	dbPath := filepath.Join(t.TempDir(), "test.sqlite")
	storeInstance, err := store.NewStore(ctx, logger, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer storeInstance.Close()

	// Add 3 non-reusable /32 ANE blocks
	for _, cidr := range []string{"10.0.1.1/32", "10.0.1.2/32", "10.0.1.3/32"} {
		if err := storeInstance.AddCIDR(ctx, network, cidr, store.WithReusable(false)); err != nil {
			t.Fatalf("Failed to add non-reusable CIDR %s: %v", cidr, err)
		}
	}

	// Allocate pods on 10.0.1.1/32 and 10.0.1.2/32; leave 10.0.1.3/32 unallocated (0 allocated IPs)
	for i, expectedCIDR := range []string{"10.0.1.1/32", "10.0.1.2/32"} {
		_, cidr, err := storeInstance.AllocateIP(ctx, store.AllocateIPParams{
			Network:       network,
			InterfaceName: "eth0",
			ContainerID:   fmt.Sprintf("ane-pod-%d", i+1),
			IPFamily:      store.IPv4,
		})
		if err != nil || cidr != expectedCIDR {
			t.Fatalf("Expected allocation on %s, got cidr=%s, err=%v", expectedCIDR, cidr, err)
		}
	}

	// Out-of-band GCE deletion: both 10.0.1.2/32 (1 allocated IP) and 10.0.1.3/32 (0 allocated IPs)
	// were deleted in GCE and replaced by 10.0.1.4/32 and 10.0.1.5/32.
	for _, cidr := range []string{"10.0.1.4/32", "10.0.1.5/32"} {
		if err := storeInstance.AddCIDR(ctx, network, cidr, store.WithReusable(false)); err != nil {
			t.Fatalf("Failed to add replacement CIDR %s: %v", cidr, err)
		}
	}

	nnc := &nncv1.NodeNetworkConfig{
		ObjectMeta: metav1.ObjectMeta{Name: nodeName, ResourceVersion: "1"},
		Spec: nncv1.NodeNetworkConfigSpec{
			Allocations: []nncv1.Allocation{
				{Network: network, Pods: 3},
			},
		},
		Status: nncv1.NodeNetworkConfigStatus{
			PodCIDRs: []nncv1.PodCIDR{
				{Network: network, CIDR: "10.0.1.1/32", ReusePolicy: nncv1.PodCIDRTypeNonReusable},
				{Network: network, CIDR: "10.0.1.4/32", ReusePolicy: nncv1.PodCIDRTypeNonReusable},
				{Network: network, CIDR: "10.0.1.5/32", ReusePolicy: nncv1.PodCIDRTypeNonReusable},
			},
		},
	}

	patchCalled := false
	var patchedData []byte
	mockInterface := &mockNodeNetworkConfigInterface{
		getFunc: func(_ context.Context, _ string, _ metav1.GetOptions) (*nncv1.NodeNetworkConfig, error) {
			return nnc, nil
		},
		patchFunc: func(_ context.Context, _ string, _ types.PatchType, data []byte, _ metav1.PatchOptions, _ ...string) (*nncv1.NodeNetworkConfig, error) {
			patchCalled = true
			patchedData = data
			return nnc, nil
		},
	}
	mockClient := &mockClientset{networkingV1: &mockNetworkingV1{nncInterface: mockInterface}}

	m := NewMonitor(MonitorConfig{
		Logger:                  logger,
		NNCClient:               mockClient,
		Store:                   storeInstance,
		NodeName:                nodeName,
		GetPendingRequestsCount: func(_ string) int { return 0 },
	})

	// First sync:
	// - 10.0.1.3/32 (allocated_ips == 0) should be immediately deleted
	// - 10.0.1.2/32 (allocated_ips == 1) should be marked Unknown
	if err := m.syncAll(ctx); err != nil {
		t.Fatalf("syncAll failed: %v", err)
	}

	_, exists, err := storeInstance.GetCIDRBlock(ctx, "10.0.1.3/32", network)
	if err != nil || exists {
		t.Errorf("Expected unallocated out-of-sync non-reusable block 10.0.1.3/32 to be deleted immediately, exists=%v, err=%v", exists, err)
	}

	b2ID, exists, err := storeInstance.GetCIDRBlock(ctx, "10.0.1.2/32", network)
	if err != nil || !exists {
		t.Fatalf("Expected allocated out-of-sync non-reusable block 10.0.1.2/32 to remain while pod is active, exists=%v, err=%v", exists, err)
	}
	blocks, err := storeInstance.GetNonDeletingCIDRBlocks(ctx, network, store.IPv4)
	if err != nil {
		t.Fatalf("GetNonDeletingCIDRBlocks failed: %v", err)
	}
	var b2State store.CidrBlockState
	for _, b := range blocks {
		if b.ID == b2ID {
			b2State = b.State
		}
	}
	if b2State != store.StateUnknown {
		t.Errorf("Expected 10.0.1.2/32 state to be Unknown, got %q", b2State)
	}

	// Usage total should be 3 (only counting 10.0.1.1/32, 10.0.1.4/32, 10.0.1.5/32), and no scale-up triggered
	usage, err := storeInstance.GetIPUsage(ctx, network, store.IPv4)
	if err != nil {
		t.Fatalf("GetIPUsage failed: %v", err)
	}
	if usage.Total != 3 || usage.Allocated != 1 {
		t.Errorf("Expected usage Total=3, Allocated=1, got Total=%d, Allocated=%d", usage.Total, usage.Allocated)
	}
	if patchCalled {
		t.Errorf("Expected no NNC patch on first sync, but patchCalled=true with %s", string(patchedData))
	}

	// Now pod on 10.0.1.2/32 exits -> ReleaseIPByOwner transitions non-reusable block to Deleting
	if _, err := storeInstance.ReleaseIPByOwner(ctx, network, "ane-pod-2", "eth0", 0); err != nil {
		t.Fatalf("ReleaseIPByOwner failed: %v", err)
	}

	// Second sync: reconcileDeletingBlocks (Case A: !inStatus) deletes 10.0.1.2/32 from SQLite
	// without adding to releasableCIDRs or reducing Pods below 3.
	if err := m.syncAll(ctx); err != nil {
		t.Fatalf("syncAll second run failed: %v", err)
	}

	_, exists, err = storeInstance.GetCIDRBlock(ctx, "10.0.1.2/32", network)
	if err != nil || exists {
		t.Errorf("Expected 10.0.1.2/32 to be deleted from DB after pod exited, exists=%v, err=%v", exists, err)
	}
	if patchCalled {
		t.Errorf("Expected no NNC patch when deleting out-of-sync non-reusable block, but patchCalled=true with %s", string(patchedData))
	}
}

// TestMonitor_OutOfSyncCIDR_InitialBootstrap guards node bootstrap: the initial
// block comes from the CNI config and exists in SQLite before CCM has published
// any status at all. An empty, never-published status must not be read as
// "every local block is gone"; the initial block stays and the first capacity
// request still goes out.
func TestMonitor_OutOfSyncCIDR_InitialBootstrap(t *testing.T) {
	ctx := context.Background()
	logger := logr.Discard()
	network := "test-network"
	nodeName := "test-node"

	dbPath := filepath.Join(t.TempDir(), "test.sqlite")
	storeInstance, err := store.NewStore(ctx, logger, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer storeInstance.Close()

	// Initial CIDR block added by CNI/IPAM engine during node bootstrap
	if err := storeInstance.AddCIDR(ctx, network, "10.0.1.0/28"); err != nil {
		t.Fatalf("Failed to add initial CIDR: %v", err)
	}

	// Status is empty during initial bootstrap
	nnc := &nncv1.NodeNetworkConfig{
		ObjectMeta: metav1.ObjectMeta{Name: nodeName, ResourceVersion: "1"},
		Spec: nncv1.NodeNetworkConfigSpec{
			Allocations: []nncv1.Allocation{
				{Network: network, Pods: 0},
			},
		},
		Status: nncv1.NodeNetworkConfigStatus{
			PodCIDRs: []nncv1.PodCIDR{},
		},
	}

	patchCalled := false
	var patchedData []byte
	mockInterface := &mockNodeNetworkConfigInterface{
		getFunc: func(_ context.Context, _ string, _ metav1.GetOptions) (*nncv1.NodeNetworkConfig, error) {
			return nnc, nil
		},
		patchFunc: func(_ context.Context, _ string, _ types.PatchType, data []byte, _ metav1.PatchOptions, _ ...string) (*nncv1.NodeNetworkConfig, error) {
			patchCalled = true
			patchedData = data
			return nnc, nil
		},
	}
	mockClient := &mockClientset{networkingV1: &mockNetworkingV1{nncInterface: mockInterface}}

	m := NewMonitor(MonitorConfig{
		Logger:                  logger,
		NNCClient:               mockClient,
		Store:                   storeInstance,
		NodeName:                nodeName,
		GetPendingRequestsCount: func(_ string) int { return 0 },
	})

	if err := m.syncAll(ctx); err != nil {
		t.Fatalf("syncAll failed: %v", err)
	}

	// The initial CIDR must not be deleted.
	_, exists, err := storeInstance.GetCIDRBlock(ctx, "10.0.1.0/28", network)
	if err != nil || !exists {
		t.Fatalf("Initial bootstrap CIDR must not be deleted when status is empty, exists: %v, err: %v", exists, err)
	}

	// Initial capacity is bootstrapped to 16 pods.
	if !patchCalled {
		t.Fatal("Expected NNC patch to be called for bootstrap allocation")
	}
	var patch struct {
		Spec nncv1.NodeNetworkConfigSpec `json:"spec"`
	}
	if err := json.Unmarshal(patchedData, &patch); err != nil {
		t.Fatalf("Failed to unmarshal patch: %v", err)
	}
	if len(patch.Spec.Allocations) == 0 || patch.Spec.Allocations[0].Pods != 16 {
		t.Errorf("Expected bootstrap allocations pods to be 16, got %+v", patch.Spec.Allocations)
	}
}

// TestMonitor_OutOfSyncCIDR_EmptyStatus verifies how an NNC status with no pod
// CIDRs for the network is interpreted, based on the NNC Ready condition
// written by CCM's status controller:
//   - no Ready condition: status never published (bootstrap) -> local blocks
//     are left alone.
//   - Ready=False: a GCE mutation is in flight or failed -> local blocks are
//     left alone.
//   - Ready=True: CCM has synced GCE and it holds no ranges -> every local
//     block is out of sync.
func TestMonitor_OutOfSyncCIDR_EmptyStatus(t *testing.T) {
	network := "test-network"
	nodeName := "test-node"

	tests := []struct {
		name             string
		conditions       []metav1.Condition
		expectReconciled bool
	}{
		{
			name:       "no Ready condition: bootstrap, blocks untouched",
			conditions: nil,
		},
		{
			name: "Ready=False (Updating): mutation in flight, blocks untouched",
			conditions: []metav1.Condition{{
				Type:   string(nncv1.NodeNetworkConfigConditionReady),
				Status: metav1.ConditionFalse,
				Reason: "Updating",
			}},
		},
		{
			name: "Ready=True: GCE holds no ranges, blocks reconciled as out of sync",
			conditions: []metav1.Condition{{
				Type:   string(nncv1.NodeNetworkConfigConditionReady),
				Status: metav1.ConditionTrue,
				Reason: string(nncv1.NodeNetworkConfigReadyReason),
			}},
			expectReconciled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			logger := logr.Discard()

			dbPath := filepath.Join(t.TempDir(), "test.sqlite")
			storeInstance, err := store.NewStore(ctx, logger, dbPath)
			if err != nil {
				t.Fatalf("Failed to create store: %v", err)
			}
			defer storeInstance.Close()

			// One block with a pod on it, one block without.
			for _, cidr := range []string{"10.0.1.0/28", "10.0.2.0/28"} {
				if err := storeInstance.AddCIDR(ctx, network, cidr); err != nil {
					t.Fatalf("Failed to add CIDR %s: %v", cidr, err)
				}
			}
			if _, cidr, err := storeInstance.AllocateIP(ctx, store.AllocateIPParams{
				Network: network, InterfaceName: "eth0", ContainerID: "pod-1", IPFamily: store.IPv4,
			}); err != nil || cidr != "10.0.1.0/28" {
				t.Fatalf("Expected allocation on 10.0.1.0/28, got cidr=%s, err=%v", cidr, err)
			}
			allocatedID, _, err := storeInstance.GetCIDRBlock(ctx, "10.0.1.0/28", network)
			if err != nil {
				t.Fatalf("GetCIDRBlock failed: %v", err)
			}

			nnc := &nncv1.NodeNetworkConfig{
				ObjectMeta: metav1.ObjectMeta{Name: nodeName, ResourceVersion: "1"},
				Spec: nncv1.NodeNetworkConfigSpec{
					Allocations: []nncv1.Allocation{{Network: network, Pods: 32}},
				},
				Status: nncv1.NodeNetworkConfigStatus{
					PodCIDRs:   []nncv1.PodCIDR{},
					Conditions: tt.conditions,
				},
			}
			mockInterface := &mockNodeNetworkConfigInterface{
				getFunc: func(_ context.Context, _ string, _ metav1.GetOptions) (*nncv1.NodeNetworkConfig, error) {
					return nnc, nil
				},
				patchFunc: func(_ context.Context, _ string, _ types.PatchType, _ []byte, _ metav1.PatchOptions, _ ...string) (*nncv1.NodeNetworkConfig, error) {
					return nnc, nil
				},
			}
			mockClient := &mockClientset{networkingV1: &mockNetworkingV1{nncInterface: mockInterface}}

			m := NewMonitor(MonitorConfig{
				Logger:                  logger,
				NNCClient:               mockClient,
				Store:                   storeInstance,
				NodeName:                nodeName,
				GetPendingRequestsCount: func(_ string) int { return 0 },
			})
			if err := m.syncAll(ctx); err != nil {
				t.Fatalf("syncAll failed: %v", err)
			}

			_, emptyExists, err := storeInstance.GetCIDRBlock(ctx, "10.0.2.0/28", network)
			if err != nil {
				t.Fatalf("GetCIDRBlock failed: %v", err)
			}
			blocks, err := storeInstance.GetNonDeletingCIDRBlocks(ctx, network, store.IPv4)
			if err != nil {
				t.Fatalf("GetNonDeletingCIDRBlocks failed: %v", err)
			}
			var allocatedState store.CidrBlockState
			for _, b := range blocks {
				if b.ID == allocatedID {
					allocatedState = b.State
				}
			}
			usage, err := storeInstance.GetIPUsage(ctx, network, store.IPv4)
			if err != nil {
				t.Fatalf("GetIPUsage failed: %v", err)
			}

			if tt.expectReconciled {
				if emptyExists {
					t.Errorf("Expected empty out-of-sync block 10.0.2.0/28 to be deleted")
				}
				if allocatedState != store.StateUnknown {
					t.Errorf("Expected allocated out-of-sync block 10.0.1.0/28 to be Unknown, got %q", allocatedState)
				}
				if usage.Total != 0 {
					t.Errorf("Expected usage.Total = 0 once all blocks are out of sync, got %d", usage.Total)
				}
				return
			}
			if !emptyExists {
				t.Errorf("Expected 10.0.2.0/28 to be retained while status is unpublished")
			}
			if allocatedState != store.StateReady {
				t.Errorf("Expected 10.0.1.0/28 to remain Ready while status is unpublished, got %q", allocatedState)
			}
			if usage.Total != 32 {
				t.Errorf("Expected usage.Total = 32 while status is unpublished, got %d", usage.Total)
			}
		})
	}
}
