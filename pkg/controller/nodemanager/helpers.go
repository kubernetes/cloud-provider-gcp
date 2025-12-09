package nodemanager

import (
	"k8s.io/apimachinery/pkg/api/meta"
)

// isObjectInProviderConfig checks if an object belongs to a specific provider config.
func isObjectInProviderConfig(obj interface{}, providerConfigName string) bool {
	metaObj, err := meta.Accessor(obj)
	if err != nil {
		return false
	}
	return metaObj.GetLabels()[ProviderConfigLabelKey] == providerConfigName
}

// providerConfigFilteredList filters a list of objects by provider config name.
func providerConfigFilteredList(items []interface{}, providerConfigName string) []interface{} {
	var filtered []interface{}
	for _, item := range items {
		if isObjectInProviderConfig(item, providerConfigName) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}
