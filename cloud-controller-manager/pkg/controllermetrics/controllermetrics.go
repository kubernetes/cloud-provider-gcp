package controllermetrics

import (
	"sync"

	"k8s.io/component-base/metrics"
	"k8s.io/component-base/metrics/legacyregistry"
)

var registerControllerMetrics sync.Once

var (
	// WorkqueueDroppedObjects is a counter for number of times an object errors enough to be dropped from the workqueue
	// label is the name of the workqueue
	WorkqueueDroppedObjects = metrics.NewCounterVec(
		&metrics.CounterOpts{
			Name:           "workqueue_dropped_objects",
			Help:           "Number of times objects have errored enough to be dropped from the workqueue.",
			StabilityLevel: metrics.ALPHA,
		},
		[]string{"name"},
	)
)

func init() {
	registerControllerMetrics.Do(func() {
		legacyregistry.MustRegister(WorkqueueDroppedObjects)
	})
}
