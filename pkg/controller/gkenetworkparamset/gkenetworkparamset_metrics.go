package gkenetworkparamset

import (
	"sync"

	"k8s.io/component-base/metrics"
	"k8s.io/component-base/metrics/legacyregistry"
)

// GKENetworkParamSetSubsystem - subsystem name used for GKE Network Param Sets
const GKENetworkParamSetSubsystem = "gkenetworkparamset_controller"

var (
	gnpObjects = metrics.NewGaugeVec(
		&metrics.GaugeOpts{
			Subsystem:      GKENetworkParamSetSubsystem,
			Name:           "gnp_object_total",
			Help:           "Gauge measuring number of GKENetworkParamSet objects",
			StabilityLevel: metrics.ALPHA,
		},
		[]string{"networkName", "status", "type"},
	)
	fetchSubnetErrs = metrics.NewCounter(
		&metrics.CounterOpts{
			Subsystem:      GKENetworkParamSetSubsystem,
			Name:           "fetch_subnet_errors_total",
			Help:           "Number of Errors for fetching subnetwork for GNP sync",
			StabilityLevel: metrics.ALPHA,
		},
	)
)

var registerGNPMetrics sync.Once

// registerGKENetworkParamSetMetrics registers GKENetworkParamSet metrics.
func registerGKENetworkParamSetMetrics() {
	registerGNPMetrics.Do(func() {
		legacyregistry.MustRegister(gnpObjects)
		legacyregistry.MustRegister(fetchSubnetErrs)
	})
}
