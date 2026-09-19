package filteredinformer

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/tools/cache"
)

const (
	providerConfigLabel = "tenancy.gke.io/provider-config"
)

// NewLabelIndexFunc returns a cache.IndexFunc that indexes objects by the value of the specified label key.
func NewLabelIndexFunc(labelKey string) cache.IndexFunc {
	return func(obj any) ([]string, error) {
		metaObj, err := meta.Accessor(obj)
		if err != nil {
			return nil, fmt.Errorf("object has no meta: %w", err)
		}
		labels := metaObj.GetLabels()
		if val, ok := labels[labelKey]; ok {
			return []string{val}, nil
		}
		return nil, nil
	}
}

// isObjectMatchingValue checks if an object matches the specific filter key and value.
// It unpacks DeletedFinalStateUnknown objects to ensure deleted events are evaluated correctly.
func isObjectMatchingValue(obj any, filterKey, filterValue string, allowMissing bool) bool {
	if d, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		obj = d.Obj
	}
	metaObj, err := meta.Accessor(obj)
	if err != nil {
		return false
	}
	val, ok := metaObj.GetLabels()[filterKey]
	return MatchValue(val, ok, filterValue, allowMissing)
}

// getFilteredListByValue filters a list of objects by the filter key and value.
func getFilteredListByValue(items []any, filterKey, filterValue string, allowMissing bool) []any {
	var filtered []any
	for _, item := range items {
		if isObjectMatchingValue(item, filterKey, filterValue, allowMissing) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

// MatchValue checks if the actual value matches the expected value,
// or if the label is missing and missing is allowed.
func MatchValue(val string, ok bool, expectedVal string, allowMissing bool) bool {
	if !ok {
		return allowMissing
	}
	return val == expectedVal
}
