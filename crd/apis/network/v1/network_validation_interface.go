package v1

import (
	"context"
)

// NetworkValidationClient knows how to get/list network and get/list interface objects.
type NetworkValidationClient interface {
	// GetNetworkInterface find the interfaceList object.
	GetNetworkInterface(ctx context.Context, networkName string, network *Network)
	// ListNetwork list all the network under the networkName.
	ListNetworks(ctx context.Context, networkList *NetworkList)
	// ListNetworkInterface list all the interface resources under the networkinterface namespace
	ListNetworkInterfaces(ctx context.Context, networkInterfaceList *NetworkInterfaceList)
}
