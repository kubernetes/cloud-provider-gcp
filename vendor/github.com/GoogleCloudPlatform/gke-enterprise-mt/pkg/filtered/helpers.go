/*
Copyright 2026 The Kubernetes Authors.
*/

package filtered

// MatchValue checks if the specified value matches the expected filter value.
func MatchValue(value string, exists bool, filterValue string, allowMissing bool) bool {
	if allowMissing {
		return !exists || value == filterValue
	}
	return exists && value == filterValue
}


