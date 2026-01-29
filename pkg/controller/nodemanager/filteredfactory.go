package nodemanager

import (
	"sync"

	"k8s.io/client-go/informers"
	coordinationinformers "k8s.io/client-go/informers/coordination"
	coordinationv1 "k8s.io/client-go/informers/coordination/v1"
	coreinformers "k8s.io/client-go/informers/core"
	corev1 "k8s.io/client-go/informers/core/v1"
	coordinationv1listers "k8s.io/client-go/listers/coordination/v1"
	v1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

// =============================================================================
// 1. The Factory Entry Point
// =============================================================================

// FilteredSharedInformerFactory wraps the standard factory.
// It embeds the interface so all non-overridden methods (Start, WaitForCacheSync)
// pass through to the underlying factory automatically.
type FilteredSharedInformerFactory struct {
	informers.SharedInformerFactory // Embedding handles Start(), WaitForCacheSync(), etc.
	filterKey                       string
	filterValue                     string
	allowMissing                    bool

	mu        sync.Mutex
	informers []*FilteredInformer
}

func NewFilteredSharedInformerFactory(parent informers.SharedInformerFactory, key, value string, allowMissing bool) *FilteredSharedInformerFactory {
	return &FilteredSharedInformerFactory{
		SharedInformerFactory: parent,
		filterKey:             key,
		filterValue:           value,
		allowMissing:          allowMissing,
	}
}

func (f *FilteredSharedInformerFactory) RegisterInformer(inf *FilteredInformer) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.informers = append(f.informers, inf)
}

func (f *FilteredSharedInformerFactory) Cleanup() {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, inf := range f.informers {
		inf.Cleanup()
	}
	f.informers = nil
}

// OVERRIDE 1: Core (Nodes, Pods)
func (f *FilteredSharedInformerFactory) Core() coreinformers.Interface {
	return &filteredCoreWrapper{
		Interface: f.SharedInformerFactory.Core(),
		factory:   f,
	}
}

// OVERRIDE 2: Coordination (Leases) - Required for Node Lifecycle Controller
func (f *FilteredSharedInformerFactory) Coordination() coordinationinformers.Interface {
	return &filteredCoordinationWrapper{
		Interface: f.SharedInformerFactory.Coordination(),
		factory:   f,
	}
}

// OPTIONAL: Apps (DaemonSets). Usually, DaemonSets are global defs and don't need filtering.
// If you don't override this method, it calls f.SharedInformerFactory.Apps() directly (Pass-through).

// =============================================================================
// 2. The Core Chain (Nodes & Pods)
// =============================================================================

type filteredCoreWrapper struct {
	coreinformers.Interface
	factory *FilteredSharedInformerFactory
}

func (w *filteredCoreWrapper) V1() corev1.Interface {
	return &filteredCoreV1Wrapper{
		Interface: w.Interface.V1(),
		factory:   w.factory,
	}
}

type filteredCoreV1Wrapper struct {
	corev1.Interface
	factory *FilteredSharedInformerFactory
}

// Intercept Nodes()
func (w *filteredCoreV1Wrapper) Nodes() corev1.NodeInformer {
	return &filteredNodeInformer{
		NodeInformer: w.Interface.Nodes(),
		factory:      w.factory,
	}
}

// Intercept Pods() - Optional but recommended for scale
func (w *filteredCoreV1Wrapper) Pods() corev1.PodInformer {
	return &filteredPodInformer{
		PodInformer: w.Interface.Pods(),
		factory:     w.factory,
	}
}

// =============================================================================
// 3. The Coordination Chain (Leases)
// =============================================================================

type filteredCoordinationWrapper struct {
	coordinationinformers.Interface
	factory *FilteredSharedInformerFactory
}

func (w *filteredCoordinationWrapper) V1() coordinationv1.Interface {
	return &filteredCoordinationV1Wrapper{
		Interface: w.Interface.V1(),
		factory:   w.factory,
	}
}

type filteredCoordinationV1Wrapper struct {
	coordinationv1.Interface
	factory *FilteredSharedInformerFactory
}

// Intercept Leases()
func (w *filteredCoordinationV1Wrapper) Leases() coordinationv1.LeaseInformer {
	return &filteredLeaseInformer{
		LeaseInformer: w.Interface.Leases(),
		factory:       w.factory,
	}
}

// =============================================================================
// 4. The Final Informer Wrappers (The Payloads)
// =============================================================================

// --- NODE INFORMER ---
type filteredNodeInformer struct {
	corev1.NodeInformer
	factory *FilteredSharedInformerFactory
}

// This is where the magic happens. We wrap the result in FilteredInformer.
func (i *filteredNodeInformer) Informer() cache.SharedIndexInformer {
	inf := NewFilteredInformer(i.NodeInformer.Informer(), i.factory.filterKey, i.factory.filterValue, i.factory.allowMissing)
	i.factory.RegisterInformer(inf)
	return inf
}

func (i *filteredNodeInformer) Lister() v1listers.NodeLister {
	return v1listers.NewNodeLister(i.Informer().GetIndexer())
}

// --- POD INFORMER ---
type filteredPodInformer struct {
	corev1.PodInformer
	factory *FilteredSharedInformerFactory
}

func (i *filteredPodInformer) Informer() cache.SharedIndexInformer {
	// NOTE: Pods might need a different filter key/value strategy!
	// (e.g. checking spec.nodeName against a list of allowed nodes)
	// For now, this assumes Pods have the same label as the tenant.
	inf := NewFilteredInformer(i.PodInformer.Informer(), i.factory.filterKey, i.factory.filterValue, i.factory.allowMissing)
	i.factory.RegisterInformer(inf)
	return inf
}

func (i *filteredPodInformer) Lister() v1listers.PodLister {
	return v1listers.NewPodLister(i.Informer().GetIndexer())
}

// --- LEASE INFORMER ---
type filteredLeaseInformer struct {
	coordinationv1.LeaseInformer
	factory *FilteredSharedInformerFactory
}

func (i *filteredLeaseInformer) Informer() cache.SharedIndexInformer {
	// Leases in kube-node-lease often map 1:1 to nodes.
	// Filtering logic here should ensure we only see leases for our nodes.
	inf := NewFilteredInformer(i.LeaseInformer.Informer(), i.factory.filterKey, i.factory.filterValue, i.factory.allowMissing)
	i.factory.RegisterInformer(inf)
	return inf
}

func (i *filteredLeaseInformer) Lister() coordinationv1listers.LeaseLister {
	return coordinationv1listers.NewLeaseLister(i.Informer().GetIndexer())
}
