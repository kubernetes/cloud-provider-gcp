/*
Copyright 2026 The Kubernetes Authors.
*/

package gketenantcontrollers

import (
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

type filteredInformer struct {
	cache.SharedIndexInformer
	providerConfigLabel string
	providerConfigName  string
	allowMissing        bool
	registrations       []cache.ResourceEventHandlerRegistration
}

func newFilteredInformer(parent cache.SharedIndexInformer, key, value string, allowMissing bool) *filteredInformer {
	return &filteredInformer{
		SharedIndexInformer: parent,
		providerConfigLabel: key,
		providerConfigName:  value,
		allowMissing:        allowMissing,
	}
}

func (f *filteredInformer) AddEventHandler(handler cache.ResourceEventHandler) (cache.ResourceEventHandlerRegistration, error) {
	reg, err := f.SharedIndexInformer.AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: f.filterFunc,
		Handler:    handler,
	})
	if err == nil {
		f.registrations = append(f.registrations, reg)
	}
	return reg, err
}

func (f *filteredInformer) AddEventHandlerWithResyncPeriod(handler cache.ResourceEventHandler, resyncPeriod time.Duration) (cache.ResourceEventHandlerRegistration, error) {
	reg, err := f.SharedIndexInformer.AddEventHandlerWithResyncPeriod(cache.FilteringResourceEventHandler{
		FilterFunc: f.filterFunc,
		Handler:    handler,
	}, resyncPeriod)
	if err == nil {
		f.registrations = append(f.registrations, reg)
	}
	return reg, err
}

func (f *filteredInformer) Cleanup() {
	for _, reg := range f.registrations {
		_ = f.SharedIndexInformer.RemoveEventHandler(reg)
	}
	f.registrations = nil
}

func (f *filteredInformer) filterFunc(obj interface{}) bool {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return false
	}
	// ONLY pass the object if it has the exact label key and value, OR if allowMissing is true and the label is missing
	val, ok := accessor.GetLabels()[f.providerConfigLabel]
	var result bool
	if f.allowMissing {
		result = !ok || val == f.providerConfigName
	} else {
		result = ok && val == f.providerConfigName
	}
	klog.Infof("FilterFunc: Obj=%q, LabelKey=%q, ExpectedVal=%q, ActualVal=%q (exists=%t), AllowMissing=%t -> Accepted=%t", accessor.GetName(), f.providerConfigLabel, f.providerConfigName, val, ok, f.allowMissing, result)
	return result
}

func (f *filteredInformer) GetStore() cache.Store {
	return &providerConfigFilteredCache{
		Indexer:            f.SharedIndexInformer.GetIndexer(),
		providerConfigName: f.providerConfigName,
		allowMissing:       f.allowMissing,
	}
}

func (f *filteredInformer) GetIndexer() cache.Indexer {
	return &providerConfigFilteredCache{
		Indexer:            f.SharedIndexInformer.GetIndexer(),
		providerConfigName: f.providerConfigName,
		allowMissing:       f.allowMissing,
	}
}
