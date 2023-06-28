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
			Help:           "Gauge measuring number of GKENetworkParamSet objects.",
			StabilityLevel: metrics.ALPHA,
		},
		[]string{"status", "type"},
	)
)

var registerGNPMetrics sync.Once

// registerGKENetworkParamSetMetrics registers GKENetworkParamSet metrics.
func registerGKENetworkParamSetMetrics() {
	registerGNPMetrics.Do(func() {
		legacyregistry.MustRegister(gnpObjects)
	})
}
