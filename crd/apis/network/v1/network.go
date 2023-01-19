package v1

// DefaultNetworkIfEmpty takes a string corresponding to a network name and makes
// sure that if it is empty then it is set to the default network. This comes
// from the idea that a network is like a namespace, where an empty network is
// the same as the default. Use before comparisons of networks.
func DefaultNetworkIfEmpty(s string) string {
	if s == "" {
		return DefaultNetworkName
	}
	return s
}
