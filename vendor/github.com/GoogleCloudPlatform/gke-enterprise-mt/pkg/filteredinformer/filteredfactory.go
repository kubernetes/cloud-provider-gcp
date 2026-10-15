package filteredinformer

import (
	"sync"

	coordinationinformers "k8s.io/client-go/informers/coordination"
	coordinationv1 "k8s.io/client-go/informers/coordination/v1"
	coreinformers "k8s.io/client-go/informers/core"
	corev1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/informers"
	coordinationv1listers "k8s.io/client-go/listers/coordination/v1"
	v1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

// FilteredSharedInformerFactory wraps the standard SharedInformerFactory to provide filtered views
// of informers. It supports typed informer signatures introduced in client-go 1.37+.
type FilteredSharedInformerFactory struct {
	informers.SharedInformerFactory
	filterKey    string
	filterValue  string
	allowMissing bool

	mu            sync.Mutex
	informers     []*ProviderConfigFilteredInformer
	nodeInformer  cache.SharedIndexInformer
	leaseInformer cache.SharedIndexInformer
}

// NewFilteredSharedInformerFactory creates a new FilteredSharedInformerFactory.
func NewFilteredSharedInformerFactory(parent informers.SharedInformerFactory, key, value string, allowMissing bool) *FilteredSharedInformerFactory {
	return &FilteredSharedInformerFactory{
		SharedInformerFactory: parent,
		filterKey:             key,
		filterValue:           value,
		allowMissing:          allowMissing,
	}
}

// Cleanup cleans up all registered informers.
func (f *FilteredSharedInformerFactory) Cleanup() {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, inf := range f.informers {
		inf.Cleanup()
	}
	f.informers = nil
	f.nodeInformer = nil
	f.leaseInformer = nil
}

// Core overrides the standard Core() method to return filtered core informers.
func (f *FilteredSharedInformerFactory) Core() coreinformers.Interface {
	return &FilteredCoreWrapper{
		Interface: f.SharedInformerFactory.Core(),
		factory:   f,
	}
}

// Coordination overrides the standard Coordination() method to return filtered coordination informers.
func (f *FilteredSharedInformerFactory) Coordination() coordinationinformers.Interface {
	return &FilteredCoordinationWrapper{
		Interface: f.SharedInformerFactory.Coordination(),
		factory:   f,
	}
}

// --- Core Chain ---

// FilteredCoreWrapper wraps the core informers to apply filtering.
type FilteredCoreWrapper struct {
	coreinformers.Interface
	factory *FilteredSharedInformerFactory
}

// V1 returns the core v1 informers.
func (w *FilteredCoreWrapper) V1() corev1.Interface {
	return &FilteredCoreV1Wrapper{
		Interface: w.Interface.V1(),
		factory:   w.factory,
	}
}

// FilteredCoreV1Wrapper wraps the core v1 informers to apply filtering.
type FilteredCoreV1Wrapper struct {
	corev1.Interface
	factory *FilteredSharedInformerFactory
}

// Nodes returns a filtered TypedNodeInformer.
// Note: client-go 1.37+ returns TypedNodeInformer instead of the untyped NodeInformer.
func (w *FilteredCoreV1Wrapper) Nodes() corev1.TypedNodeInformer {
	return &FilteredNodeInformer{
		TypedNodeInformer: w.Interface.Nodes(),
		factory:           w.factory,
	}
}

// --- Coordination Chain ---

// FilteredCoordinationWrapper wraps the coordination informers to apply filtering.
type FilteredCoordinationWrapper struct {
	coordinationinformers.Interface
	factory *FilteredSharedInformerFactory
}

// V1 returns the coordination v1 informers.
func (w *FilteredCoordinationWrapper) V1() coordinationv1.Interface {
	return &FilteredCoordinationV1Wrapper{
		Interface: w.Interface.V1(),
		factory:   w.factory,
	}
}

// FilteredCoordinationV1Wrapper wraps the coordination v1 informers to apply filtering.
type FilteredCoordinationV1Wrapper struct {
	coordinationv1.Interface
	factory *FilteredSharedInformerFactory
}

// Leases returns a filtered TypedLeaseInformer.
// Note: client-go 1.37+ returns TypedLeaseInformer instead of the untyped LeaseInformer.
func (w *FilteredCoordinationV1Wrapper) Leases() coordinationv1.TypedLeaseInformer {
	return &FilteredLeaseInformer{
		TypedLeaseInformer: w.Interface.Leases(),
		factory:            w.factory,
	}
}

// FilteredNodeInformer wraps TypedNodeInformer to return a filtered informer.
type FilteredNodeInformer struct {
	corev1.TypedNodeInformer
	factory *FilteredSharedInformerFactory
}

// Informer returns the filtered SharedIndexInformer for nodes.
func (i *FilteredNodeInformer) Informer() cache.SharedIndexInformer {
	i.factory.mu.Lock()
	defer i.factory.mu.Unlock()
	if i.factory.nodeInformer != nil {
		return i.factory.nodeInformer
	}
	inf := NewFilteredInformer(i.TypedNodeInformer.Informer(), i.factory.filterKey, i.factory.filterValue, i.factory.allowMissing)
	i.factory.informers = append(i.factory.informers, inf)
	i.factory.nodeInformer = inf
	return inf
}

// Lister returns the filtered NodeLister.
func (i *FilteredNodeInformer) Lister() v1listers.NodeLister {
	return v1listers.NewNodeLister(i.Informer().GetIndexer())
}

// TypedInformer returns the filtered NodeIndexInformer.
// Note: This method is required by TypedNodeInformer in client-go 1.37+.
func (i *FilteredNodeInformer) TypedInformer() corev1.NodeIndexInformer {
	return corev1.ToNodeIndexInformer(i.Informer())
}

// FilteredLeaseInformer wraps TypedLeaseInformer to return a filtered informer.
type FilteredLeaseInformer struct {
	coordinationv1.TypedLeaseInformer
	factory *FilteredSharedInformerFactory
}

// Informer returns the filtered SharedIndexInformer for leases.
func (i *FilteredLeaseInformer) Informer() cache.SharedIndexInformer {
	i.factory.mu.Lock()
	defer i.factory.mu.Unlock()
	if i.factory.leaseInformer != nil {
		return i.factory.leaseInformer
	}
	inf := NewFilteredInformer(i.TypedLeaseInformer.Informer(), i.factory.filterKey, i.factory.filterValue, i.factory.allowMissing)
	i.factory.informers = append(i.factory.informers, inf)
	i.factory.leaseInformer = inf
	return inf
}

// Lister returns the filtered LeaseLister.
func (i *FilteredLeaseInformer) Lister() coordinationv1listers.LeaseLister {
	return coordinationv1listers.NewLeaseLister(i.Informer().GetIndexer())
}

// TypedInformer returns the filtered LeaseIndexInformer.
// Note: This method is required by TypedLeaseInformer in client-go 1.37+.
func (i *FilteredLeaseInformer) TypedInformer() coordinationv1.LeaseIndexInformer {
	return coordinationv1.ToLeaseIndexInformer(i.Informer())
}
