package filteredinformer

import (
	"k8s.io/client-go/tools/cache"
)

// providerConfigFilteredCache implements cache.Store and cache.Indexer with custom filtering.
type providerConfigFilteredCache struct {
	cache.Indexer
	filterKey    string
	filterValue  string
	allowMissing bool
}

// ByIndex returns a list of objects that match the given index name and indexed value.
func (pc *providerConfigFilteredCache) ByIndex(indexName, indexedValue string) ([]any, error) {
	items, err := pc.Indexer.ByIndex(indexName, indexedValue)
	if err != nil {
		return nil, err
	}
	return getFilteredListByValue(items, pc.filterKey, pc.filterValue, pc.allowMissing), nil
}

// Index returns a list of objects that match the given index name and indexed value.
func (pc *providerConfigFilteredCache) Index(indexName string, obj any) ([]any, error) {
	items, err := pc.Indexer.Index(indexName, obj)
	if err != nil {
		return nil, err
	}
	return getFilteredListByValue(items, pc.filterKey, pc.filterValue, pc.allowMissing), nil
}

// IndexKeys returns a list of keys matching the filter.
func (pc *providerConfigFilteredCache) IndexKeys(indexName, indexedValue string) ([]string, error) {
	keys, err := pc.Indexer.IndexKeys(indexName, indexedValue)
	if err != nil {
		return nil, err
	}

	filteredKeys := make([]string, 0, len(keys))
	for _, key := range keys {
		item, exists, err := pc.Indexer.GetByKey(key)
		if err != nil {
			return nil, err
		}
		if exists && isObjectMatchingValue(item, pc.filterKey, pc.filterValue, pc.allowMissing) {
			filteredKeys = append(filteredKeys, key)
		}
	}
	return filteredKeys, nil
}

// List returns a list of objects matching the filter.
func (pc *providerConfigFilteredCache) List() []any {
	// The label index cannot answer allowMissing queries: objects that do not
	// carry filterKey are not indexed under any value, yet they must match when
	// allowMissing is set. Only take the index fast path when the index alone is
	// guaranteed to produce the complete answer.
	if !pc.allowMissing {
		items, err := pc.Indexer.ByIndex(pc.filterKey, pc.filterValue)
		if err == nil {
			return items
		}
	}
	// Fallback to the slower method if the index is not available or cannot
	// express the filter.
	return getFilteredListByValue(pc.Indexer.List(), pc.filterKey, pc.filterValue, pc.allowMissing)
}

// ListKeys returns a list of keys matching the filter.
func (pc *providerConfigFilteredCache) ListKeys() []string {
	items := pc.List()
	keys := make([]string, 0, len(items))
	for _, item := range items {
		if key, err := cache.MetaNamespaceKeyFunc(item); err == nil {
			keys = append(keys, key)
		}
	}
	return keys
}

// Get returns an object matching the filter.
func (pc *providerConfigFilteredCache) Get(obj any) (item any, exists bool, err error) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return nil, false, err
	}
	return pc.GetByKey(key)
}

// GetByKey returns an object matching the filter.
func (pc *providerConfigFilteredCache) GetByKey(key string) (item any, exists bool, err error) {
	item, exists, err = pc.Indexer.GetByKey(key)
	if !exists || err != nil {
		return nil, exists, err
	}
	if isObjectMatchingValue(item, pc.filterKey, pc.filterValue, pc.allowMissing) {
		return item, true, nil
	}
	return nil, false, nil
}

// LastStoreSyncResourceVersion returns the last resource version from the underlying store.
// This is a new method required by client-go 1.36. It is used for tracking objects in the
// underlying store and determining whether to resume a watch or not. Since filtered cache is a
// projection over the shared underlying store, its resource version must stay in line with the
// underlying cache.
func (pc *providerConfigFilteredCache) LastStoreSyncResourceVersion() string {
	return pc.Indexer.LastStoreSyncResourceVersion()
}

// Bookmark bookmarks the given resource version in the underlying store.
// This is a new method required by client-go 1.36. Like LastStoreSyncResourceVersion, bookmarking
// ensures progress tracking and watch resumption remain synchronized with the underlying physical
// watch stream regardless of provider config filtering.
func (pc *providerConfigFilteredCache) Bookmark(rv string) {
	pc.Indexer.Bookmark(rv)
}
