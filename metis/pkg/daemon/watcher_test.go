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
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	nncv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/nodenetworkconfig/v1"
	nncfake "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/clientset/versioned/fake"
	"github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/informers/externalversions"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/metis/pkg/store"
)

func TestWatcher_Success(t *testing.T) {
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
	dbPath := filepath.Join(tempDir, "metis_watcher_success_test.sqlite")
	storeInstance, err := store.NewStore(context.Background(), logger, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer storeInstance.Close()

	nodeName := "test-node"
	network := "test-network"

	// Add network to DB so periodic sync can find it
	if err := storeInstance.AddCIDR(context.Background(), network, "10.0.1.0/24"); err != nil {
		t.Fatalf("Failed to add CIDR to DB: %v", err)
	}

	mockNNC := &nncv1.NodeNetworkConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:            nodeName,
			ResourceVersion: "1",
		},
		Spec: nncv1.NodeNetworkConfigSpec{
			Allocations: []nncv1.Allocation{
				{Network: network, Pods: 32},
			},
		},
	}

	nncClient := nncfake.NewSimpleClientset(mockNNC)
	nncInformerFactory := externalversions.NewSharedInformerFactory(nncClient, 0)
	nncInformer := nncInformerFactory.Networking().V1().NodeNetworkConfigs()

	w := NewWatcher(logger, nncClient, nncInformer, storeInstance, nodeName, nil)
	w.syncHandler = syncHandler

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	nncInformerFactory.Start(ctx.Done())

	doneCh := make(chan struct{})
	go func() {
		w.Run(ctx, 1)
		close(doneCh)
	}()

	if !cache.WaitForCacheSync(ctx.Done(), nncInformer.Informer().HasSynced) {
		t.Fatal("Timed out waiting for caches to sync")
	}

	mockNNCUpdated := mockNNC.DeepCopy()
	mockNNCUpdated.ResourceVersion = "2"
	mockNNCUpdated.Spec.Allocations[0].Pods = 64

	_, err = nncClient.NetworkingV1().NodeNetworkConfigs().Update(ctx, mockNNCUpdated, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("Failed to update NodeNetworkConfig: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	if !processed[network] {
		t.Errorf("Expected %s to be processed", network)
	}
	mu.Unlock()

	cancel() // Trigger shutdown

	select {
	case <-doneCh:
		// Success, it shut down!
	case <-time.After(2 * time.Second):
		t.Fatal("Watcher failed to shut down in time")
	}

	if !w.queue.ShuttingDown() {
		t.Error("Expected queue to be shutting down")
	}
}

func TestWatcher_Retry(t *testing.T) {
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
	dbPath := filepath.Join(tempDir, "metis_watcher_retry_test.sqlite")
	storeInstance, err := store.NewStore(context.Background(), logger, dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer storeInstance.Close()

	w := NewWatcher(logger, nil, nil, storeInstance, "test-node", nil)
	w.syncHandler = syncHandler
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go w.Run(ctx, 1)

	w.queue.Add("test-network")

	// Wait for processing and retry
	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if processCount < 2 {
		t.Errorf("Expected at least 2 attempts, got %d", processCount)
	}
}

func TestWatcher_SyncCIDR(t *testing.T) {
	logger := logr.Discard()
	network := "test-network"
	nodeName := "test-node"

	tests := []struct {
		desc   string
		blocks []struct {
			cidr  string
			state string
		}
		mockNNC             *nncv1.NodeNetworkConfig
		injectErr           error
		cidrToCheck         string
		expectedExists      bool
		expectedOnCIDRAdded bool
		expectedErr         bool
		closeDB             bool
		lockDB              bool
	}{
		{
			desc:   "Add new CIDR from NNC status",
			blocks: nil,
			mockNNC: &nncv1.NodeNetworkConfig{
				ObjectMeta: metav1.ObjectMeta{Name: nodeName},
				Status: nncv1.NodeNetworkConfigStatus{
					PodCIDRs: []nncv1.PodCIDR{
						{CIDR: "10.0.1.0/28", Network: network, Condition: &metav1.Condition{Status: metav1.ConditionTrue}},
					},
				},
			},
			cidrToCheck:         "10.0.1.0/28",
			expectedExists:      true,
			expectedOnCIDRAdded: true,
		},
		{
			desc:   "Ignore unready CIDR from NNC status",
			blocks: nil,
			mockNNC: &nncv1.NodeNetworkConfig{
				ObjectMeta: metav1.ObjectMeta{Name: nodeName},
				Status: nncv1.NodeNetworkConfigStatus{
					PodCIDRs: []nncv1.PodCIDR{
						{CIDR: "10.0.1.0/28", Network: network, Condition: &metav1.Condition{Status: metav1.ConditionFalse}},
					},
				},
			},
			cidrToCheck:         "10.0.1.0/28",
			expectedExists:      false,
			expectedOnCIDRAdded: false,
		},
		{
			desc: "Cleanup deleting CIDR not in NNC status",
			blocks: []struct {
				cidr  string
				state string
			}{
				{"10.0.1.0/28", "Deleting"},
			},
			mockNNC: &nncv1.NodeNetworkConfig{
				ObjectMeta: metav1.ObjectMeta{Name: nodeName},
				Spec: nncv1.NodeNetworkConfigSpec{
					ReleasableCIDRs: []nncv1.PodCIDR{},
				},
				Status: nncv1.NodeNetworkConfigStatus{
					PodCIDRs: []nncv1.PodCIDR{}, // Empty status
				},
			},
			cidrToCheck:         "10.0.1.0/28",
			expectedExists:      false,
			expectedOnCIDRAdded: false,
		},
		{
			desc:                "API server error on Get",
			injectErr:           fmt.Errorf("api server error"),
			cidrToCheck:         "10.0.1.0/28",
			expectedExists:      false,
			expectedOnCIDRAdded: false,
			expectedErr:         true,
		},
		{
			desc: "Store error on GetCIDRBlockByCIDR (addCIDR)",
			mockNNC: &nncv1.NodeNetworkConfig{
				ObjectMeta: metav1.ObjectMeta{Name: nodeName},
				Status: nncv1.NodeNetworkConfigStatus{
					PodCIDRs: []nncv1.PodCIDR{
						{CIDR: "10.0.1.0/28", Network: network, Condition: &metav1.Condition{Status: metav1.ConditionTrue}},
					},
				},
			},
			cidrToCheck:         "10.0.1.0/28",
			expectedExists:      false,
			expectedOnCIDRAdded: false,
			expectedErr:         true,
			closeDB:             true,
		},
		{
			desc: "Store error on GetDeletingCIDRBlocks (maybeDeleteCIDRs)",
			mockNNC: &nncv1.NodeNetworkConfig{
				ObjectMeta: metav1.ObjectMeta{Name: nodeName},
				Status: nncv1.NodeNetworkConfigStatus{
					PodCIDRs: []nncv1.PodCIDR{}, // Empty status
				},
			},
			cidrToCheck:         "10.0.1.0/28",
			expectedExists:      false,
			expectedOnCIDRAdded: false,
			expectedErr:         true,
			closeDB:             true,
		},
		{
			desc: "Store error on AddCIDR (Database Busy)",
			mockNNC: &nncv1.NodeNetworkConfig{
				ObjectMeta: metav1.ObjectMeta{Name: nodeName},
				Status: nncv1.NodeNetworkConfigStatus{
					PodCIDRs: []nncv1.PodCIDR{
						{CIDR: "10.0.1.0/28", Network: network, Condition: &metav1.Condition{Status: metav1.ConditionTrue}},
					},
				},
			},
			cidrToCheck:         "10.0.1.0/28",
			expectedExists:      false,
			expectedOnCIDRAdded: false,
			expectedErr:         true,
			lockDB:              true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			tempDir := t.TempDir()
			dbPath := filepath.Join(tempDir, "metis_watcher_sync_cidr_test.sqlite")
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
				if b.state == "Deleting" {
					blocks, err := storeInstance.GetReadyCIDRBlocksSorted(context.Background(), network)
					if err != nil || len(blocks) == 0 {
						t.Fatalf("Failed to get block ID: %v", err)
					}
					err = storeInstance.MarkCIDRBlockAsDeleting(context.Background(), blocks[0].ID)
					if err != nil {
						t.Fatalf("Failed to mark block as Deleting: %v", err)
					}
				}
			}

			mockInterface := &mockNodeNetworkConfigInterface{
				getFunc: func(ctx context.Context, name string, opts metav1.GetOptions) (*nncv1.NodeNetworkConfig, error) {
					if tc.injectErr != nil {
						return nil, tc.injectErr
					}
					return tc.mockNNC, nil
				},
			}
			mockNetV1 := &mockNetworkingV1{nncInterface: mockInterface}
			mockClient := &mockClientset{networkingV1: mockNetV1}

			onCIDRAddedCalled := false
			w := NewWatcher(logger, mockClient, nil, storeInstance, nodeName, func(net string, availableIPs int) {
				onCIDRAddedCalled = true
			})

			if tc.closeDB {
				storeInstance.DB().Close()
			}

			if tc.lockDB {
				_, err := storeInstance.DB().Exec("PRAGMA busy_timeout = 10;")
				if err != nil {
					t.Fatalf("Failed to set busy timeout: %v", err)
				}

				db, err := sql.Open("sqlite3", dbPath+"?_txlock=immediate")
				if err != nil {
					t.Fatalf("Failed to open separate DB connection: %v", err)
				}
				defer db.Close()

				tx, err := db.Begin()
				if err != nil {
					t.Fatalf("Failed to begin transaction: %v", err)
				}
				_, err = tx.Exec("UPDATE cidr_blocks SET updated_at = CURRENT_TIMESTAMP WHERE id = -1")
				if err != nil {
					t.Fatalf("Failed to lock DB: %v", err)
				}
			}

			err = w.syncCIDR(context.Background(), network)
			if tc.expectedErr {
				if err == nil {
					t.Errorf("Expected syncCIDR to fail, but it succeeded")
				}
				return
			}
			if err != nil {
				t.Fatalf("syncCIDR failed: %v", err)
			}

			_, exists, err := storeInstance.GetCIDRBlockByCIDRAndNetwork(context.Background(), tc.cidrToCheck, network)
			if err != nil {
				t.Fatalf("GetCIDRBlockByCIDRAndNetwork failed: %v", err)
			}
			if exists != tc.expectedExists {
				t.Errorf("Expected exists %v, got %v for CIDR %s", tc.expectedExists, exists, tc.cidrToCheck)
			}

			if onCIDRAddedCalled != tc.expectedOnCIDRAdded {
				t.Errorf("Expected onCIDRAddedCalled %v, got %v", tc.expectedOnCIDRAdded, onCIDRAddedCalled)
			}
		})
	}
}
