package providerconfig

import (
	"time"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"
)

// NewInformer creates a new ProviderConfig CR informer.
func NewInformer(dynamicClientSet dynamic.Interface, resyncDuration time.Duration) cache.SharedIndexInformer {
	factory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClientSet, resyncDuration)
	return factory.ForResource(ProviderConfigGVR).Informer()
}
