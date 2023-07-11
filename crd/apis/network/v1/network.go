package v1

// DefaultNetworkIfEmpty takes a string corresponding to a network name and makes
// sure that if it is empty then it is set to the default network. This comes
// from the idea that a network is like a namespace, where an empty network is
// the same as the default. Use before comparisons of networks.
func DefaultNetworkIfEmpty(s string) string {
	if s == "" {
		return DefaultPodNetworkName
	}
	return s
}

// IsDefaultNetwork takes a network name and returns if it is a default network.
// Both DefaultNetworkName and DefaultPodNetworkName are considered as default network for compatibility purposes.
// DefaultNetworkName will eventually be removed.
func IsDefaultNetwork(networkName string) bool {
	return networkName == DefaultNetworkName || networkName == DefaultPodNetworkName
}

// IsDefaultNetworkOrEmpty takes a network name and returns if it is a default network or empty.
// Both DefaultNetworkName and DefaultPodNetworkName are considered as default network for compatibility purposes.
// DefaultNetworkName will eventually be removed.
func IsDefaultNetworkOrEmpty(networkName string) bool {
	return networkName == "" || IsDefaultNetwork(networkName)
}

// InUse is true if the network is referenced by NetworkInterface or Pod objects.
func (n *Network) InUse() bool {
	if n.Annotations == nil {
		return false
	}
	val, ok := n.Annotations[NetworkInUseAnnotationKey]
	return ok && val == NetworkInUseAnnotationValTrue
}
