package v1

import "fmt"

// InterfaceName returns the expected host interface for this Network
// If vlanID is specified, the expected tagged interface Name is returned
// otherwise the user specified interfaceName is returned
func (n *Network) InterfaceName() (string, error) {
	if n.Spec.NodeInterfaceMatcher.InterfaceName == nil || *n.Spec.NodeInterfaceMatcher.InterfaceName == "" {
		return "", fmt.Errorf("invalid network %s: network.spec.nodeInterfaceMatcher.InterfaceName cannot be nil or empty", n.Name)
	}
	hostInterface := n.Spec.NodeInterfaceMatcher.InterfaceName

	if n.Spec.L2NetworkConfig == nil || n.Spec.L2NetworkConfig.VlanID == nil {
		return *hostInterface, nil
	}

	return fmt.Sprintf("%s.%d", *hostInterface, *n.Spec.L2NetworkConfig.VlanID), nil
}

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
