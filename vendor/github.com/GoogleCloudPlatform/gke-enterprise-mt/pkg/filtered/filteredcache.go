/*
Copyright 2026 The Kubernetes Authors.
*/

package filtered

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

// FilteredCache implements cache.Store and cache.Indexer with custom filtering.
// It wraps the standard cache.Indexer and filters the results based on the filterKey and filterValue.
// if allowMissing is true, it will allow objects that do not have the filterKey.
type FilteredCache struct {
	cache.Indexer
	filterKey    string
	filterValue  string
	allowMissing bool
}

func (obj *FilteredCache) ByIndex(indexName, indexedValue string) ([]interface{}, error) {
	items, err := obj.Indexer.ByIndex(indexName, indexedValue)
	if err != nil {
		return nil, err
	}
	return getFilteredListByValue(items, obj.filterKey, obj.filterValue, obj.allowMissing), nil
}

func (obj *FilteredCache) Index(indexName string, item interface{}) ([]interface{}, error) {
	items, err := obj.Indexer.Index(indexName, item)
	if err != nil {
		return nil, err
	}
	return getFilteredListByValue(items, obj.filterKey, obj.filterValue, obj.allowMissing), nil
}

func (obj *FilteredCache) List() []interface{} {
	return getFilteredListByValue(obj.Indexer.List(), obj.filterKey, obj.filterValue, obj.allowMissing)
}

func (obj *FilteredCache) ListKeys() []string {
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

func (obj *FilteredCache) Get(item interface{}) (interface{}, bool, error) {
	key, err := cache.MetaNamespaceKeyFunc(item)
	if err != nil {
		klog.Errorf("Get: failed to get key for item %v: %v", item, err)
		return nil, false, err
	}
	return obj.GetByKey(key)
}

func (obj *FilteredCache) GetByKey(key string) (item interface{}, exists bool, err error) {
	item, exists, err = obj.Indexer.GetByKey(key)
	if !exists || err != nil {
		return nil, exists, err
	}
	if isObjectMatchingValue(item, obj.filterKey, obj.filterValue, obj.allowMissing) {
		return item, true, nil
	}
	return nil, false, nil
}

// isObjectMatchingValue checks if an object matches to the specific value.
// If allowMissing is true, objects without the required label are also considered as belonging to the specific value.
func isObjectMatchingValue(obj interface{}, filterKey, filterValue string, allowMissing bool) bool {
	metaObj, err := meta.Accessor(obj)
	if err != nil {
		klog.Errorf("isObjectMatchingValue: failed to get meta accessor for object %v: %v", obj, err)
		return false
	}
	val, ok := metaObj.GetLabels()[filterKey]
	return MatchValue(val, ok, filterValue, allowMissing)
}

// getFilteredListByValue filters a list of objects by the filter key and value.
func getFilteredListByValue(items []interface{}, filterKey, filterValue string, allowMissing bool) []interface{} {
	var filtered []interface{}
	for _, item := range items {
		if isObjectMatchingValue(item, filterKey, filterValue, allowMissing) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}