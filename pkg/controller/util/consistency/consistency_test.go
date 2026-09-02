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

package consistency

import (
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

type fakeRVGetter struct {
	mu sync.RWMutex
	rv string
}

func (f *fakeRVGetter) LastStoreSyncResourceVersion() string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.rv
}

func (f *fakeRVGetter) setRV(rv string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rv = rv
}

func TestConsistencyStore_BasicReadinessAndStall(t *testing.T) {
	nodeGR := schema.GroupResource{Group: "", Resource: "nodes"}
	getter := &fakeRVGetter{rv: "100"}
	store := NewConsistencyStore(map[schema.GroupResource]LastSyncRVGetter{
		nodeGR: getter,
	})

	key := types.NamespacedName{Name: "node-1"}

	// 1. Initial state is ready since no writes recorded
	require.NoError(t, store.EnsureReady(key))

	// 2. Record a write at RV 105
	store.WroteAt(key, types.UID("uid-1"), nodeGR, "105")

	// 3. EnsureReady should return a ConsistencyError because store is at RV 100 < 105
	err := store.EnsureReady(key)
	require.Error(t, err)
	var consistencyErr *ConsistencyError
	require.True(t, errors.As(err, &consistencyErr))
	assert.Equal(t, key, consistencyErr.Key)
	assert.Equal(t, nodeGR, consistencyErr.GroupResource)
	assert.Equal(t, "105", consistencyErr.ExpectedRV)
	assert.Equal(t, "100", consistencyErr.ActualRV)

	// 4. Advance getter RV to 105 (caught up)
	getter.setRV("105")
	assert.NoError(t, store.EnsureReady(key))

	// 5. Subsequent EnsureReady remains ready (expectations satisfied and cleared)
	assert.NoError(t, store.EnsureReady(key))
}

func TestConsistencyStore_MultipleResourcesForSameKey(t *testing.T) {
	gnpGR := schema.GroupResource{Group: "networking.gke.io", Resource: "gkenetworkparamsets"}
	netGR := schema.GroupResource{Group: "networking.gke.io", Resource: "networks"}

	gnpGetter := &fakeRVGetter{rv: "10"}
	netGetter := &fakeRVGetter{rv: "20"}

	store := NewConsistencyStore(map[schema.GroupResource]LastSyncRVGetter{
		gnpGR: gnpGetter,
		netGR: netGetter,
	})

	key := types.NamespacedName{Name: "default"}

	// Record write for both GNP and Network
	store.WroteAt(key, types.UID("gnp-uid"), gnpGR, "15")
	store.WroteAt(key, types.UID("net-uid"), netGR, "25")

	// Neither is ready
	err := store.EnsureReady(key)
	require.Error(t, err)

	// Catch up GNP only
	gnpGetter.setRV("15")
	err = store.EnsureReady(key)
	require.Error(t, err)
	var cErr *ConsistencyError
	require.True(t, errors.As(err, &cErr))
	assert.Equal(t, netGR, cErr.GroupResource)

	// Catch up Network
	netGetter.setRV("30")
	assert.NoError(t, store.EnsureReady(key))
}

func TestConsistencyStore_Clear(t *testing.T) {
	nodeGR := schema.GroupResource{Group: "", Resource: "nodes"}
	getter := &fakeRVGetter{rv: "10"}
	store := NewConsistencyStore(map[schema.GroupResource]LastSyncRVGetter{
		nodeGR: getter,
	})

	key := types.NamespacedName{Name: "node-1"}

	// Record write with UID
	store.WroteAt(key, types.UID("uid-1"), nodeGR, "20")
	require.Error(t, store.EnsureReady(key))

	// Clear with mismatched UID should not clear
	store.Clear(key, types.UID("other-uid"))
	require.Error(t, store.EnsureReady(key))

	// Clear with matching UID should clear
	store.Clear(key, types.UID("uid-1"))
	assert.NoError(t, store.EnsureReady(key))

	// Record write again and Clear with empty UID should clear unconditionally
	store.WroteAt(key, types.UID("uid-2"), nodeGR, "30")
	require.Error(t, store.EnsureReady(key))
	store.Clear(key, "")
	assert.NoError(t, store.EnsureReady(key))
}

func TestConsistencyStore_NonNumericRV(t *testing.T) {
	nodeGR := schema.GroupResource{Group: "", Resource: "nodes"}
	getter := &fakeRVGetter{rv: "abc"}
	store := NewConsistencyStore(map[schema.GroupResource]LastSyncRVGetter{
		nodeGR: getter,
	})

	key := types.NamespacedName{Name: "node-1"}

	store.WroteAt(key, types.UID("uid-1"), nodeGR, "xyz")
	require.Error(t, store.EnsureReady(key))

	getter.setRV("xyz")
	assert.NoError(t, store.EnsureReady(key))
}

func TestConsistencyStore_ConcurrentAccess(t *testing.T) {
	nodeGR := schema.GroupResource{Group: "", Resource: "nodes"}
	getter := &fakeRVGetter{rv: "0"}
	store := NewConsistencyStore(map[schema.GroupResource]LastSyncRVGetter{
		nodeGR: getter,
	})

	key := types.NamespacedName{Name: "node-1"}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(3)
		go func(val int) {
			defer wg.Done()
			store.WroteAt(key, types.UID("uid-1"), nodeGR, "10")
		}(i)
		go func(val int) {
			defer wg.Done()
			_ = store.EnsureReady(key)
		}(i)
		go func(val int) {
			defer wg.Done()
			store.Clear(key, types.UID("uid-1"))
		}(i)
	}
	wg.Wait()
}
