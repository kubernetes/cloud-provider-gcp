package ipam

import (
	"sync"

	"k8s.io/component-base/metrics"
	"k8s.io/component-base/metrics/legacyregistry"
)

// nodeIpamSubsystem - subsystem name used for Node IPAM Controller
const nodeIpamSubsystem = "node_ipam_controller"

var (
	multiNetworkNodes = metrics.NewGaugeVec(
		&metrics.GaugeOpts{
			Subsystem:      nodeIpamSubsystem,
			Name:           "multinetwork_node_total",
			Help:           "Gauge measuring number of multinetworking nodes that have subscribed to a given network",
			StabilityLevel: metrics.ALPHA,
		},
		[]string{"network"},
	)
)

var registerMetrics sync.Once

// registerCloudCidrAllocatorMetrics registers Cloud CIDR Allocator metrics.
func registerCloudCidrAllocatorMetrics() {
	registerMetrics.Do(func() {
		legacyregistry.MustRegister(multiNetworkNodes)
	})
}
