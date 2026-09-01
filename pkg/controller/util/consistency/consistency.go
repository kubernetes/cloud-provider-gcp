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
	"fmt"
	"strconv"
	"sync"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

// LastSyncRVGetter is an interface for retrieving the latest resource version that the store has seen.
// cache.Store in client-go implements this interface via LastStoreSyncResourceVersion().
type LastSyncRVGetter interface {
	LastStoreSyncResourceVersion() string
}

// ConsistencyError is returned by EnsureReady when the store's sync resource version lags behind the expected resource version.
type ConsistencyError struct {
	Key           types.NamespacedName
	GroupResource schema.GroupResource
	ExpectedRV    string
	ActualRV      string
}

func (e *ConsistencyError) Error() string {
	return fmt.Sprintf("cache for %s (%s) is stale: expected RV >= %s, got %s", e.Key, e.GroupResource, e.ExpectedRV, e.ActualRV)
}

// ConsistencyStore tracks writes to resources and checks whether informers have observed them.
type ConsistencyStore interface {
	// WroteAt records that the resource identified by key, uid, and gr was written at resourceVersion.
	WroteAt(key types.NamespacedName, uid types.UID, gr schema.GroupResource, resourceVersion string)
	// EnsureReady returns an error if any tracked resource for key is not yet reflected in the informer cache.
	EnsureReady(key types.NamespacedName) error
	// Clear removes tracked resource versions for key (and optionally uid).
	Clear(key types.NamespacedName, uid types.UID)
}

type resourceExpectation struct {
	uid types.UID
	rv  string
}

type consistencyStoreImpl struct {
	mu           sync.RWMutex
	getters      map[schema.GroupResource]LastSyncRVGetter
	expectations map[types.NamespacedName]map[schema.GroupResource]resourceExpectation
}

// NewConsistencyStore returns a new ConsistencyStore initialized with the given LastSyncRVGetters.
func NewConsistencyStore(getters map[schema.GroupResource]LastSyncRVGetter) ConsistencyStore {
	storeGetters := make(map[schema.GroupResource]LastSyncRVGetter, len(getters))
	for gr, getter := range getters {
		storeGetters[gr] = getter
	}
	return &consistencyStoreImpl{
		getters:      storeGetters,
		expectations: make(map[types.NamespacedName]map[schema.GroupResource]resourceExpectation),
	}
}

// WroteAt records that a write was performed for the specified resource.
func (s *consistencyStoreImpl) WroteAt(key types.NamespacedName, uid types.UID, gr schema.GroupResource, resourceVersion string) {
	if resourceVersion == "" || resourceVersion == "0" {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	expMap, ok := s.expectations[key]
	if !ok {
		expMap = make(map[schema.GroupResource]resourceExpectation)
		s.expectations[key] = expMap
	}

	existing, exists := expMap[gr]
	if exists && existing.uid == uid {
		if isRVNewer(resourceVersion, existing.rv) {
			expMap[gr] = resourceExpectation{uid: uid, rv: resourceVersion}
		}
	} else {
		expMap[gr] = resourceExpectation{uid: uid, rv: resourceVersion}
	}
}

// EnsureReady checks whether the stores associated with the key have observed the required resource versions.
func (s *consistencyStoreImpl) EnsureReady(key types.NamespacedName) error {
	s.mu.RLock()
	expMap, ok := s.expectations[key]
	if !ok || len(expMap) == 0 {
		s.mu.RUnlock()
		return nil
	}

	for gr, exp := range expMap {
		getter, hasGetter := s.getters[gr]
		if !hasGetter || getter == nil {
			continue
		}
		actualRV := getter.LastStoreSyncResourceVersion()
		if !isStoreReady(actualRV, exp.rv) {
			s.mu.RUnlock()
			return &ConsistencyError{
				Key:           key,
				GroupResource: gr,
				ExpectedRV:    exp.rv,
				ActualRV:      actualRV,
			}
		}
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()
	if expMap, ok := s.expectations[key]; ok {
		for gr, exp := range expMap {
			getter, hasGetter := s.getters[gr]
			if hasGetter && getter != nil {
				if isStoreReady(getter.LastStoreSyncResourceVersion(), exp.rv) {
					delete(expMap, gr)
				}
			}
		}
		if len(expMap) == 0 {
			delete(s.expectations, key)
		}
	}

	return nil
}

// Clear removes recorded resource version expectations for the specified key and optional uid.
func (s *consistencyStoreImpl) Clear(key types.NamespacedName, uid types.UID) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if uid == "" {
		delete(s.expectations, key)
		return
	}

	expMap, ok := s.expectations[key]
	if !ok {
		return
	}

	for gr, exp := range expMap {
		if exp.uid == uid {
			delete(expMap, gr)
		}
	}
	if len(expMap) == 0 {
		delete(s.expectations, key)
	}
}

func isStoreReady(storeRV, expectedRV string) bool {
	if expectedRV == "" || expectedRV == "0" {
		return true
	}
	if storeRV == "" {
		return false
	}
	storeNum, err1 := strconv.ParseUint(storeRV, 10, 64)
	expNum, err2 := strconv.ParseUint(expectedRV, 10, 64)
	if err1 == nil && err2 == nil {
		return storeNum >= expNum
	}
	return storeRV == expectedRV
}

func isRVNewer(newRV, oldRV string) bool {
	newNum, err1 := strconv.ParseUint(newRV, 10, 64)
	oldNum, err2 := strconv.ParseUint(oldRV, 10, 64)
	if err1 == nil && err2 == nil {
		return newNum > oldNum
	}
	return newRV != oldRV
}
