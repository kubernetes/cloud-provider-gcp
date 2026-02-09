/*
Copyright 2026 The Kubernetes Authors.
*/

package gketenantcontrollers

import (
	"k8s.io/apimachinery/pkg/api/meta"
)

// isObjectInProviderConfig checks if an object belongs to a specific provider config.
func isObjectInProviderConfig(obj interface{}, providerConfigName string, allowMissing bool) bool {
	metaObj, err := meta.Accessor(obj)
	if err != nil {
		return false
	}
	val, ok := metaObj.GetLabels()[providerConfigLabelKey]
	var res bool
	if allowMissing {
		res = !ok || val == providerConfigName
	} else {
		res = ok && val == providerConfigName
	}
	return res
}

// providerConfigFilteredList filters a list of objects by provider config name.
func providerConfigFilteredList(items []interface{}, providerConfigName string, allowMissing bool) []interface{} {
	var filtered []interface{}
	for _, item := range items {
		if isObjectInProviderConfig(item, providerConfigName, allowMissing) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}
