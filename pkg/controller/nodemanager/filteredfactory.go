package nodemanager

import (
	"k8s.io/client-go/informers"
	coordinationinformers "k8s.io/client-go/informers/coordination"
	coordinationv1 "k8s.io/client-go/informers/coordination/v1"
	coreinformers "k8s.io/client-go/informers/core"
	corev1 "k8s.io/client-go/informers/core/v1"
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
}

func NewFilteredSharedInformerFactory(parent informers.SharedInformerFactory, key, value string) informers.SharedInformerFactory {
	return &FilteredSharedInformerFactory{
		SharedInformerFactory: parent,
		filterKey:             key,
		filterValue:           value,
	}
}

// OVERRIDE 1: Core (Nodes, Pods)
func (f *FilteredSharedInformerFactory) Core() coreinformers.Interface {
	return &filteredCoreWrapper{
		Interface:   f.SharedInformerFactory.Core(),
		filterKey:   f.filterKey,
		filterValue: f.filterValue,
	}
}

// OVERRIDE 2: Coordination (Leases) - Required for Node Lifecycle Controller
func (f *FilteredSharedInformerFactory) Coordination() coordinationinformers.Interface {
	return &filteredCoordinationWrapper{
		Interface:   f.SharedInformerFactory.Coordination(),
		filterKey:   f.filterKey,
		filterValue: f.filterValue,
	}
}

// OPTIONAL: Apps (DaemonSets). Usually, DaemonSets are global defs and don't need filtering.
// If you don't override this method, it calls f.SharedInformerFactory.Apps() directly (Pass-through).

// =============================================================================
// 2. The Core Chain (Nodes & Pods)
// =============================================================================

type filteredCoreWrapper struct {
	coreinformers.Interface
	filterKey, filterValue string
}

func (w *filteredCoreWrapper) V1() corev1.Interface {
	return &filteredCoreV1Wrapper{
		Interface:   w.Interface.V1(),
		filterKey:   w.filterKey,
		filterValue: w.filterValue,
	}
}

type filteredCoreV1Wrapper struct {
	corev1.Interface
	filterKey, filterValue string
}

// Intercept Nodes()
func (w *filteredCoreV1Wrapper) Nodes() corev1.NodeInformer {
	return &filteredNodeInformer{
		NodeInformer: w.Interface.Nodes(),
		filterKey:    w.filterKey,
		filterValue:  w.filterValue,
	}
}

// Intercept Pods() - Optional but recommended for scale
func (w *filteredCoreV1Wrapper) Pods() corev1.PodInformer {
	return &filteredPodInformer{
		PodInformer: w.Interface.Pods(),
		filterKey:   w.filterKey,
		filterValue: w.filterValue,
	}
}

// =============================================================================
// 3. The Coordination Chain (Leases)
// =============================================================================

type filteredCoordinationWrapper struct {
	coordinationinformers.Interface
	filterKey, filterValue string
}

func (w *filteredCoordinationWrapper) V1() coordinationv1.Interface {
	return &filteredCoordinationV1Wrapper{
		Interface:   w.Interface.V1(),
		filterKey:   w.filterKey,
		filterValue: w.filterValue,
	}
}

type filteredCoordinationV1Wrapper struct {
	coordinationv1.Interface
	filterKey, filterValue string
}

// Intercept Leases()
func (w *filteredCoordinationV1Wrapper) Leases() coordinationv1.LeaseInformer {
	return &filteredLeaseInformer{
		LeaseInformer: w.Interface.Leases(),
		filterKey:     w.filterKey,
		filterValue:   w.filterValue,
	}
}

// =============================================================================
// 4. The Final Informer Wrappers (The Payloads)
// =============================================================================

// --- NODE INFORMER ---
type filteredNodeInformer struct {
	corev1.NodeInformer
	filterKey, filterValue string
}

// This is where the magic happens. We wrap the result in FilteredInformer.
func (i *filteredNodeInformer) Informer() cache.SharedIndexInformer {
	return NewFilteredInformer(i.NodeInformer.Informer(), i.filterKey, i.filterValue)
}

// --- POD INFORMER ---
type filteredPodInformer struct {
	corev1.PodInformer
	filterKey, filterValue string
}

func (i *filteredPodInformer) Informer() cache.SharedIndexInformer {
	// NOTE: Pods might need a different filter key/value strategy!
	// (e.g. checking spec.nodeName against a list of allowed nodes)
	// For now, this assumes Pods have the same label as the tenant.
	return NewFilteredInformer(i.PodInformer.Informer(), i.filterKey, i.filterValue)
}

// --- LEASE INFORMER ---
type filteredLeaseInformer struct {
	coordinationv1.LeaseInformer
	filterKey, filterValue string
}

func (i *filteredLeaseInformer) Informer() cache.SharedIndexInformer {
	// Leases in kube-node-lease often map 1:1 to nodes.
	// Filtering logic here should ensure we only see leases for our nodes.
	return NewFilteredInformer(i.LeaseInformer.Informer(), i.filterKey, i.filterValue)
}
