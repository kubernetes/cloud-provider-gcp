/*
Copyright 2025 The Kubernetes Authors.
*/

package nodemanager

import (
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/tools/cache"
)

type FilteredInformer struct {
	cache.SharedIndexInformer
	providerConfigLabel string
	providerConfigName  string
	registrations       []cache.ResourceEventHandlerRegistration
}

func NewFilteredInformer(parent cache.SharedIndexInformer, key, value string) *FilteredInformer {
	return &FilteredInformer{
		SharedIndexInformer: parent,
		providerConfigLabel: key,
		providerConfigName:  value,
	}
}

func (f *FilteredInformer) AddEventHandler(handler cache.ResourceEventHandler) (cache.ResourceEventHandlerRegistration, error) {
	reg, err := f.SharedIndexInformer.AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: f.filterFunc,
		Handler:    handler,
	})
	if err == nil {
		f.registrations = append(f.registrations, reg)
	}
	return reg, err
}

func (f *FilteredInformer) AddEventHandlerWithResyncPeriod(handler cache.ResourceEventHandler, resyncPeriod time.Duration) (cache.ResourceEventHandlerRegistration, error) {
	reg, err := f.SharedIndexInformer.AddEventHandlerWithResyncPeriod(cache.FilteringResourceEventHandler{
		FilterFunc: f.filterFunc,
		Handler:    handler,
	}, resyncPeriod)
	if err == nil {
		f.registrations = append(f.registrations, reg)
	}
	return reg, err
}

func (f *FilteredInformer) Cleanup() {
	for _, reg := range f.registrations {
		_ = f.SharedIndexInformer.RemoveEventHandler(reg)
	}
	f.registrations = nil
}

func (f *FilteredInformer) filterFunc(obj interface{}) bool {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return false
	}
	// ONLY pass the object if it has the exact label key and value
	return accessor.GetLabels()[f.providerConfigLabel] == f.providerConfigName
}

func (f *FilteredInformer) GetStore() cache.Store {
	return &providerConfigFilteredCache{
		Indexer:            f.SharedIndexInformer.GetIndexer(),
		providerConfigName: f.providerConfigName,
	}
}

func (f *FilteredInformer) GetIndexer() cache.Indexer {
	return &providerConfigFilteredCache{
		Indexer:            f.SharedIndexInformer.GetIndexer(),
		providerConfigName: f.providerConfigName,
	}
}
