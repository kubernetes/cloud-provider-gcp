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
}

func NewFilteredInformer(parent cache.SharedIndexInformer, key, value string) cache.SharedIndexInformer {
	return &FilteredInformer{
		SharedIndexInformer: parent,
		providerConfigLabel: key,
		providerConfigName:  value,
	}
}

func (f *FilteredInformer) AddEventHandler(handler cache.ResourceEventHandler) (cache.ResourceEventHandlerRegistration, error) {
	return f.SharedIndexInformer.AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: f.filterFunc,
		Handler:    handler,
	})
}

func (f *FilteredInformer) AddEventHandlerWithResyncPeriod(handler cache.ResourceEventHandler, resyncPeriod time.Duration) (cache.ResourceEventHandlerRegistration, error) {
	return f.SharedIndexInformer.AddEventHandlerWithResyncPeriod(cache.FilteringResourceEventHandler{
		FilterFunc: f.filterFunc,
		Handler:    handler,
	}, resyncPeriod)
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
