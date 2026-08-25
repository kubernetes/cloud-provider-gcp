/*
Copyright 2026 The Kubernetes Authors.
*/

package utils

import (
	"fmt"
	"sync"
	"time"

	"github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/filtered"
	networkinformers "github.com/GoogleCloudPlatform/gke-networking-api/client/network/informers/externalversions"
	network "github.com/GoogleCloudPlatform/gke-networking-api/client/network/informers/externalversions/network"
	networkv1 "github.com/GoogleCloudPlatform/gke-networking-api/client/network/informers/externalversions/network/v1"
	networklisters "github.com/GoogleCloudPlatform/gke-networking-api/client/network/listers/network/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

// FilteredNetworkSharedInformerFactory wraps the standard network informer factory
// and applies tenant-specific filtering to Network and GKENetworkParamSet resources.
type FilteredNetworkSharedInformerFactory struct {
	networkinformers.SharedInformerFactory
	filterKey    string
	filterValue  string
	allowMissing bool

	mu        sync.Mutex
	informers map[string]*localFilteredInformer
}

// NewFilteredNetworkSharedInformerFactory creates a new FilteredNetworkSharedInformerFactory.
func NewFilteredNetworkSharedInformerFactory(parent networkinformers.SharedInformerFactory, key, value string, allowMissing bool) *FilteredNetworkSharedInformerFactory {
	return &FilteredNetworkSharedInformerFactory{
		SharedInformerFactory: parent,
		filterKey:             key,
		filterValue:           value,
		allowMissing:          allowMissing,
		informers:             make(map[string]*localFilteredInformer),
	}
}

// getOrCreateInformer safely fetches a cached informer, or creates it if missing.
func (f *FilteredNetworkSharedInformerFactory) getOrCreateInformer(informerType string, parent cache.SharedIndexInformer) cache.SharedIndexInformer {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Return the cached informer if it already exists
	if inf, exists := f.informers[informerType]; exists {
		return inf
	}

	// Otherwise create, cache, and return it
	inf := newLocalFilteredInformer(parent, f.filterKey, f.filterValue, f.allowMissing)
	f.informers[informerType] = inf
	return inf
}

// Cleanup unregisters all event handlers.
func (f *FilteredNetworkSharedInformerFactory) Cleanup() {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, inf := range f.informers {
		inf.Cleanup()
	}
	f.informers = make(map[string]*localFilteredInformer) // Reset the cache
}

// Networking returns a wrapped Interface.
func (f *FilteredNetworkSharedInformerFactory) Networking() network.Interface {
	return &FilteredNetworkWrapper{
		Interface: f.SharedInformerFactory.Networking(),
		factory:   f,
	}
}

// FilteredNetworkWrapper wraps network.Interface to return filtered versions.
type FilteredNetworkWrapper struct {
	network.Interface
	factory *FilteredNetworkSharedInformerFactory
}

// V1 returns a wrapped networkv1.Interface.
func (w *FilteredNetworkWrapper) V1() networkv1.Interface {
	return &FilteredNetworkV1Wrapper{
		Interface: w.Interface.V1(),
		factory:   w.factory,
	}
}

// FilteredNetworkV1Wrapper wraps networkv1.Interface to return filtered informers.
type FilteredNetworkV1Wrapper struct {
	networkv1.Interface
	factory *FilteredNetworkSharedInformerFactory
}

// Networks returns a filtered NetworkInformer.
func (w *FilteredNetworkV1Wrapper) Networks() networkv1.NetworkInformer {
	return &FilteredNetworkInformer{
		NetworkInformer: w.Interface.Networks(),
		factory:         w.factory,
	}
}

// GKENetworkParamSets returns a filtered GKENetworkParamSetInformer.
func (w *FilteredNetworkV1Wrapper) GKENetworkParamSets() networkv1.GKENetworkParamSetInformer {
	return &FilteredGKENetworkParamSetInformer{
		GKENetworkParamSetInformer: w.Interface.GKENetworkParamSets(),
		factory:                    w.factory,
	}
}

// FilteredNetworkInformer wraps networkv1.NetworkInformer.
type FilteredNetworkInformer struct {
	networkv1.NetworkInformer
	factory *FilteredNetworkSharedInformerFactory
}

// Informer returns the filtered index informer.
func (i *FilteredNetworkInformer) Informer() cache.SharedIndexInformer {
	return i.factory.getOrCreateInformer("networks", i.NetworkInformer.Informer())
}

// Lister returns the filtered lister.
func (i *FilteredNetworkInformer) Lister() networklisters.NetworkLister {
	return networklisters.NewNetworkLister(i.Informer().GetIndexer())
}

// FilteredGKENetworkParamSetInformer wraps networkv1.GKENetworkParamSetInformer.
type FilteredGKENetworkParamSetInformer struct {
	networkv1.GKENetworkParamSetInformer
	factory *FilteredNetworkSharedInformerFactory
}

// Informer returns the filtered index informer.
func (i *FilteredGKENetworkParamSetInformer) Informer() cache.SharedIndexInformer {
	return i.factory.getOrCreateInformer("gkenetworkparamsets", i.GKENetworkParamSetInformer.Informer())
}

// Lister returns the filtered lister.
func (i *FilteredGKENetworkParamSetInformer) Lister() networklisters.GKENetworkParamSetLister {
	return networklisters.NewGKENetworkParamSetLister(i.Informer().GetIndexer())
}

// localFilteredInformer implements cache.SharedIndexInformer with custom filtering.
type localFilteredInformer struct {
	cache.SharedIndexInformer
	filterKey    string
	filterValue  string
	allowMissing bool

	mu            sync.Mutex
	registrations []cache.ResourceEventHandlerRegistration
}

func newLocalFilteredInformer(parent cache.SharedIndexInformer, key, value string, allowMissing bool) *localFilteredInformer {
	return &localFilteredInformer{
		SharedIndexInformer: parent,
		filterKey:           key,
		filterValue:         value,
		allowMissing:        allowMissing,
	}
}

func (f *localFilteredInformer) AddEventHandler(handler cache.ResourceEventHandler) (cache.ResourceEventHandlerRegistration, error) {
	reg, err := f.SharedIndexInformer.AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: f.FilterFunc,
		Handler:    handler,
	})
	if err == nil {
		f.mu.Lock()
		f.registrations = append(f.registrations, reg)
		f.mu.Unlock()
	}
	return reg, err
}

func (f *localFilteredInformer) AddEventHandlerWithResyncPeriod(handler cache.ResourceEventHandler, resyncPeriod time.Duration) (cache.ResourceEventHandlerRegistration, error) {
	reg, err := f.SharedIndexInformer.AddEventHandlerWithResyncPeriod(cache.FilteringResourceEventHandler{
		FilterFunc: f.FilterFunc,
		Handler:    handler,
	}, resyncPeriod)
	if err == nil {
		f.mu.Lock()
		f.registrations = append(f.registrations, reg)
		f.mu.Unlock()
	}
	return reg, err
}

func (f *localFilteredInformer) Cleanup() {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, reg := range f.registrations {
		_ = f.SharedIndexInformer.RemoveEventHandler(reg)
	}
	f.registrations = nil
}

func (f *localFilteredInformer) FilterFunc(obj interface{}) bool {
	if deletedState, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		obj = deletedState.Obj
	}
	accessor, err := meta.Accessor(obj)
	if err != nil {
		klog.Errorf("FilterFunc: failed to get meta accessor for object %v: %v", obj, err)
		return false
	}
	val, ok := accessor.GetLabels()[f.filterKey]
	return filtered.MatchValue(val, ok, f.filterValue, f.allowMissing)
}

func (f *localFilteredInformer) GetStore() cache.Store {
	return &localFilteredCache{
		Indexer:      f.SharedIndexInformer.GetIndexer(),
		filterKey:    f.filterKey,
		filterValue:  f.filterValue,
		allowMissing: f.allowMissing,
	}
}

func (f *localFilteredInformer) GetIndexer() cache.Indexer {
	return &localFilteredCache{
		Indexer:      f.SharedIndexInformer.GetIndexer(),
		filterKey:    f.filterKey,
		filterValue:  f.filterValue,
		allowMissing: f.allowMissing,
	}
}

// localFilteredCache wraps standard Indexer to filter lists/gets for the tenant.
type localFilteredCache struct {
	cache.Indexer
	filterKey    string
	filterValue  string
	allowMissing bool
}

func (obj *localFilteredCache) ByIndex(indexName, indexedValue string) ([]interface{}, error) {
	items, err := obj.Indexer.ByIndex(indexName, indexedValue)
	if err != nil {
		return nil, err
	}
	return getFilteredListByValue(items, obj.filterKey, obj.filterValue, obj.allowMissing), nil
}

func (obj *localFilteredCache) Index(indexName string, item interface{}) ([]interface{}, error) {
	items, err := obj.Indexer.Index(indexName, item)
	if err != nil {
		return nil, err
	}
	return getFilteredListByValue(items, obj.filterKey, obj.filterValue, obj.allowMissing), nil
}

func (obj *localFilteredCache) List() []interface{} {
	return getFilteredListByValue(obj.Indexer.List(), obj.filterKey, obj.filterValue, obj.allowMissing)
}

func (obj *localFilteredCache) ListKeys() []string {
	items := obj.List()
	var keys []string
	for _, item := range items {
		if key, err := cache.MetaNamespaceKeyFunc(item); err == nil {
			keys = append(keys, key)
		} else {
			klog.Errorf("ListKeys: failed to get key for item %v: %v", item, err)
		}
	}
	return keys
}

func (obj *localFilteredCache) Get(item interface{}) (interface{}, bool, error) {
	key, err := cache.MetaNamespaceKeyFunc(item)
	if err != nil {
		klog.Errorf("Get: failed to get key for item %v: %v", item, err)
		return nil, false, err
	}
	return obj.GetByKey(key)
}

func (obj *localFilteredCache) GetByKey(key string) (item interface{}, exists bool, err error) {
	item, exists, err = obj.Indexer.GetByKey(key)
	if !exists || err != nil {
		return nil, exists, err
	}
	if isObjectMatchingValue(item, obj.filterKey, obj.filterValue, obj.allowMissing) {
		return item, true, nil
	}
	return nil, false, nil
}

func isObjectMatchingValue(obj interface{}, filterKey, filterValue string, allowMissing bool) bool {
	if deletedState, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		obj = deletedState.Obj
	}
	metaObj, err := meta.Accessor(obj)
	if err != nil {
		klog.Errorf("isObjectMatchingValue: failed to get meta accessor for object %v: %v", obj, err)
		return false
	}
	val, ok := metaObj.GetLabels()[filterKey]
	return filtered.MatchValue(val, ok, filterValue, allowMissing)
}

func getFilteredListByValue(items []interface{}, filterKey, filterValue string, allowMissing bool) []interface{} {
	var filteredItems []interface{}
	for _, item := range items {
		if isObjectMatchingValue(item, filterKey, filterValue, allowMissing) {
			filteredItems = append(filteredItems, item)
		}
	}
	return filteredItems
}

func (obj *localFilteredCache) IndexKeys(indexName, indexedValue string) ([]string, error) {
	keys, err := obj.Indexer.IndexKeys(indexName, indexedValue)
	if err != nil {
		return nil, err
	}
	var filteredKeys []string
	for _, key := range keys {
		item, exists, err := obj.Indexer.GetByKey(key)
		if err != nil || !exists {
			continue
		}
		if isObjectMatchingValue(item, obj.filterKey, obj.filterValue, obj.allowMissing) {
			filteredKeys = append(filteredKeys, key)
		}
	}
	return filteredKeys, nil
}

func (obj *localFilteredCache) ListIndexFuncValues(indexName string) []string {
	indexers := obj.GetIndexers()
	indexFunc, ok := indexers[indexName]
	if !ok {
		return nil
	}
	values := make(map[string]struct{})
	for _, item := range obj.List() {
		if vals, err := indexFunc(item); err == nil {
			for _, v := range vals {
				values[v] = struct{}{}
			}
		}
	}
	var res []string
	for v := range values {
		res = append(res, v)
	}
	return res
}

func (obj *localFilteredCache) Add(item interface{}) error {
	return fmt.Errorf("read-only filtered cache")
}

func (obj *localFilteredCache) Update(item interface{}) error {
	return fmt.Errorf("read-only filtered cache")
}

func (obj *localFilteredCache) Delete(item interface{}) error {
	return fmt.Errorf("read-only filtered cache")
}

func (obj *localFilteredCache) Replace(list []interface{}, resourceVersion string) error {
	return fmt.Errorf("read-only filtered cache")
}

func (obj *localFilteredCache) Resync() error {
	return fmt.Errorf("read-only filtered cache")
}
