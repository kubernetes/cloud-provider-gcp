// Package filteredinformer implements informer with provider config filtering.
package filteredinformer

import (
	"sync"
	"time"

	"k8s.io/client-go/tools/cache"
)

// ProviderConfigFilteredInformer wraps a SharedIndexInformer to provide a filtered view.
type ProviderConfigFilteredInformer struct {
	cache.SharedIndexInformer
	filterKey    string
	filterValue  string
	allowMissing bool

	mu            sync.Mutex
	registrations []cache.ResourceEventHandlerRegistration
}

var globalMu sync.Mutex

// NewFilteredInformer creates a new generic FilteredInformer (internally named ProviderConfigFilteredInformer for compatibility).
func NewFilteredInformer(informer cache.SharedIndexInformer, filterKey, filterValue string, allowMissing bool) *ProviderConfigFilteredInformer {
	globalMu.Lock()
	defer globalMu.Unlock()
	indexers := informer.GetIndexer().GetIndexers()
	if indexers != nil {
		if _, ok := indexers[filterKey]; !ok {
			informer.AddIndexers(cache.Indexers{filterKey: NewLabelIndexFunc(filterKey)})
		}
	}
	return &ProviderConfigFilteredInformer{
		SharedIndexInformer: informer,
		filterKey:           filterKey,
		filterValue:         filterValue,
		allowMissing:        allowMissing,
	}
}

// NewProviderConfigFilteredInformer creates a new ProviderConfigFilteredInformer (legacy constructor).
func NewProviderConfigFilteredInformer(informer cache.SharedIndexInformer, providerConfigName string) *ProviderConfigFilteredInformer {
	return NewFilteredInformer(informer, providerConfigLabel, providerConfigName, false)
}

// AddEventHandler adds an event handler that only processes events matching the filter.
func (i *ProviderConfigFilteredInformer) AddEventHandler(handler cache.ResourceEventHandler) (cache.ResourceEventHandlerRegistration, error) {
	reg, err := i.SharedIndexInformer.AddEventHandler(
		cache.FilteringResourceEventHandler{
			FilterFunc: i.filterFunc,
			Handler:    handler,
		},
	)
	if err != nil {
		return reg, err
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	i.registrations = append(i.registrations, reg)
	return reg, nil
}

// AddEventHandlerWithResyncPeriod adds an event handler with resync period.
func (i *ProviderConfigFilteredInformer) AddEventHandlerWithResyncPeriod(handler cache.ResourceEventHandler, resyncPeriod time.Duration) (cache.ResourceEventHandlerRegistration, error) {
	reg, err := i.SharedIndexInformer.AddEventHandlerWithResyncPeriod(
		cache.FilteringResourceEventHandler{
			FilterFunc: i.filterFunc,
			Handler:    handler,
		},
		resyncPeriod,
	)
	if err != nil {
		return reg, err
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	i.registrations = append(i.registrations, reg)
	return reg, nil
}

// AddEventHandlerWithOptions adds an event handler with options.
func (i *ProviderConfigFilteredInformer) AddEventHandlerWithOptions(handler cache.ResourceEventHandler, options cache.HandlerOptions) (cache.ResourceEventHandlerRegistration, error) {
	reg, err := i.SharedIndexInformer.AddEventHandlerWithOptions(
		cache.FilteringResourceEventHandler{
			FilterFunc: i.filterFunc,
			Handler:    handler,
		},
		options,
	)
	if err != nil {
		return reg, err
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	i.registrations = append(i.registrations, reg)
	return reg, nil
}

// filterFunc filters objects based on the configured key and value.
func (i *ProviderConfigFilteredInformer) filterFunc(obj any) bool {
	return isObjectMatchingValue(obj, i.filterKey, i.filterValue, i.allowMissing)
}

// GetStore returns a Store that only stores objects matching the filter.
func (i *ProviderConfigFilteredInformer) GetStore() cache.Store {
	return &providerConfigFilteredCache{
		Indexer:      i.SharedIndexInformer.GetIndexer(),
		filterKey:    i.filterKey,
		filterValue:  i.filterValue,
		allowMissing: i.allowMissing,
	}
}

// GetIndexer returns an Indexer that only indexes objects matching the filter.
func (i *ProviderConfigFilteredInformer) GetIndexer() cache.Indexer {
	return &providerConfigFilteredCache{
		Indexer:      i.SharedIndexInformer.GetIndexer(),
		filterKey:    i.filterKey,
		filterValue:  i.filterValue,
		allowMissing: i.allowMissing,
	}
}

// RemoveEventHandler removes an event handler. The handle is also dropped from
// the tracked set so that Cleanup does not try to remove it a second time.
func (i *ProviderConfigFilteredInformer) RemoveEventHandler(handle cache.ResourceEventHandlerRegistration) error {
	i.mu.Lock()
	for idx, reg := range i.registrations {
		if reg == handle {
			i.registrations = append(i.registrations[:idx], i.registrations[idx+1:]...)
			break
		}
	}
	i.mu.Unlock()

	return i.SharedIndexInformer.RemoveEventHandler(handle)
}

// Cleanup deregisters every event handler that was added through this filtered
// informer, so the underlying shared informer stops delivering events to them.
//
// Note: the underlying RemoveEventHandler is asynchronous; it stops queueing new
// events but does not wait for already-queued events to finish executing.
// Cleanup is idempotent and safe to call concurrently.
func (i *ProviderConfigFilteredInformer) Cleanup() {
	i.mu.Lock()
	regs := i.registrations
	i.registrations = nil
	i.mu.Unlock()

	for _, reg := range regs {
		// RemoveEventHandler is documented as idempotent and thread-safe. An error
		// here means the handler is already gone, which is the desired end state.
		_ = i.SharedIndexInformer.RemoveEventHandler(reg)
	}
}

// HasSyncedChecker returns a checker that can be used to check if the informer has synced.
// Note: This is a new method required by client-go 1.36.
func (i *ProviderConfigFilteredInformer) HasSyncedChecker() cache.DoneChecker {
	return i.SharedIndexInformer.HasSyncedChecker()
}
