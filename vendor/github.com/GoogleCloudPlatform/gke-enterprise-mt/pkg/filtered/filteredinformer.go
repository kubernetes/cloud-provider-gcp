/*
Copyright 2026 The Kubernetes Authors.
*/

package filtered

import (
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

type FilteredInformer struct {
	cache.SharedIndexInformer
	filterKey     string
	filterValue   string
	allowMissing  bool
	// This is needed because the filtered handlers are added to the parent informer and need to be removed when the tenant is deleted.
	// Parent informer doesn't know about the filtered handlers for that tenant.
	registrations []cache.ResourceEventHandlerRegistration
}

func newFilteredInformer(parent cache.SharedIndexInformer, key, value string, allowMissing bool) *FilteredInformer {
	return &FilteredInformer{
		SharedIndexInformer: parent,
		filterKey:           key,
		filterValue:         value,
		allowMissing:        allowMissing,
	}
}

func (f *FilteredInformer) AddEventHandler(handler cache.ResourceEventHandler) (cache.ResourceEventHandlerRegistration, error) {
	reg, err := f.SharedIndexInformer.AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: f.FilterFunc,
		Handler:    handler,
	})
	if err == nil {
		f.registrations = append(f.registrations, reg)
	}
	return reg, err
}

func (f *FilteredInformer) AddEventHandlerWithResyncPeriod(handler cache.ResourceEventHandler, resyncPeriod time.Duration) (cache.ResourceEventHandlerRegistration, error) {
	reg, err := f.SharedIndexInformer.AddEventHandlerWithResyncPeriod(cache.FilteringResourceEventHandler{
		FilterFunc: f.FilterFunc,
		Handler:    handler,
	}, resyncPeriod)
	if err == nil {
		f.registrations = append(f.registrations, reg)
	}
	return reg, err
}

// Cleanup is a custom function to FilteredInformer. 
// It is needed to handle our very specific multi-tenant requirement of cleanly 
// unregistering handlers when a tenant shuts down without shutting down the global cache.
func (f *FilteredInformer) Cleanup() {
	for _, reg := range f.registrations {
		_ = f.SharedIndexInformer.RemoveEventHandler(reg)
	}
	f.registrations = nil
}

// FilterFunc is a custom function to FilteredInformer.
// It contains the core logic to determine if a given Kubernetes object (e.g. Node, Lease)
// belongs to the tenant by checking its labels against the configured filterKey and filterValue.
func (f *FilteredInformer) FilterFunc(obj interface{}) bool {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		klog.Errorf("FilterFunc: failed to get meta accessor for object %v: %v", obj, err)
		return false
	}
	// filter in the object if it has the exact label key and value
	// OR if allowMissing is true and the label is missing
	val, ok := accessor.GetLabels()[f.filterKey]
	result := MatchValue(val, ok, f.filterValue, f.allowMissing)
	klog.Infof("FilterFunc: Obj=%q, LabelKey=%q, ExpectedVal=%q, ActualVal=%q (exists=%t), AllowMissing=%t -> Accepted=%t", accessor.GetName(), f.filterKey, f.filterValue, val, ok, f.allowMissing, result)
	return result
}

func (f *FilteredInformer) GetStore() cache.Store {
	return &FilteredCache{
		Indexer:      f.SharedIndexInformer.GetIndexer(),
		filterKey:    f.filterKey,
		filterValue:  f.filterValue,
		allowMissing: f.allowMissing,
	}
}

func (f *FilteredInformer) GetIndexer() cache.Indexer {
	return &FilteredCache{
		Indexer:      f.SharedIndexInformer.GetIndexer(),
		filterKey:    f.filterKey,
		filterValue:  f.filterValue,
		allowMissing: f.allowMissing,
	}
}
