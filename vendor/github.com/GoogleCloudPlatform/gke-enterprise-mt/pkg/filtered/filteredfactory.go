/*
Copyright 2026 The Kubernetes Authors.
*/

// Package filtered provides a factory for filtered informers.
package filtered

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
// filterKey is the label key to filter on.
// filterValue is the label value to filter on.
// allowMissing true means objects without the filterKey will be allowed.
// WARNING: For now this only overrides Core() and Coordination() informers specifically for Nodes and Leases.
// For others, it just returns the underlying factory. We will be adding more informers soon.
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

// RegisterInformer is a custom function to FilteredSharedInformerFactory.
// It is called internally when a new FilteredInformer wrapper is created
// to keep track of it within the factory. This ensures that the factory can
// call Cleanup() on all registered wrappers when the tenant is deleted.
func (f *FilteredSharedInformerFactory) RegisterInformer(inf *FilteredInformer) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.informers = append(f.informers, inf)
}

// Cleanup is a custom function to FilteredSharedInformerFactory. 
// It is needed to handle our very specific multi-tenant requirement of cleanly 
// unregistering handlers without shutting down the global cache.
func (f *FilteredSharedInformerFactory) Cleanup() {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, inf := range f.informers {
		inf.Cleanup()
	}
	f.informers = nil
}

// OVERRIDE 1: Core (Nodes)
func (f *FilteredSharedInformerFactory) Core() coreinformers.Interface {
	return &FilteredCoreWrapper{
		Interface: f.SharedInformerFactory.Core(),
		factory:   f,
	}
}

// OVERRIDE 2: Coordination (Leases)
func (f *FilteredSharedInformerFactory) Coordination() coordinationinformers.Interface {
	return &FilteredCoordinationWrapper{
		Interface: f.SharedInformerFactory.Coordination(),
		factory:   f,
	}
}

// =============================================================================
// 2. The Core Chain (Nodes)
// =============================================================================

type FilteredCoreWrapper struct {
	coreinformers.Interface
	factory *FilteredSharedInformerFactory
}

func (w *FilteredCoreWrapper) V1() corev1.Interface {
	return &FilteredCoreV1Wrapper{
		Interface: w.Interface.V1(),
		factory:   w.factory,
	}
}

type FilteredCoreV1Wrapper struct {
	corev1.Interface
	factory *FilteredSharedInformerFactory
}

// Intercept Nodes()
func (w *FilteredCoreV1Wrapper) Nodes() corev1.NodeInformer {
	return &FilteredNodeInformer{
		NodeInformer: w.Interface.Nodes(),
		factory:      w.factory,
	}
}

// =============================================================================
// 3. The Coordination Chain (Leases)
// =============================================================================

type FilteredCoordinationWrapper struct {
	coordinationinformers.Interface
	factory *FilteredSharedInformerFactory
}

func (w *FilteredCoordinationWrapper) V1() coordinationv1.Interface {
	return &FilteredCoordinationV1Wrapper{
		Interface: w.Interface.V1(),
		factory:   w.factory,
	}
}

type FilteredCoordinationV1Wrapper struct {
	coordinationv1.Interface
	factory *FilteredSharedInformerFactory
}

// Intercept Leases()
func (w *FilteredCoordinationV1Wrapper) Leases() coordinationv1.LeaseInformer {
	return &FilteredLeaseInformer{
		LeaseInformer: w.Interface.Leases(),
		factory:       w.factory,
	}
}

// =============================================================================
// 4. The Final Informer Wrappers
// =============================================================================

// --- NODE INFORMER ---
type FilteredNodeInformer struct {
	corev1.NodeInformer
	factory *FilteredSharedInformerFactory
}

func (i *FilteredNodeInformer) Informer() cache.SharedIndexInformer {
	inf := newFilteredInformer(i.NodeInformer.Informer(), i.factory.filterKey, i.factory.filterValue, i.factory.allowMissing)
	i.factory.RegisterInformer(inf)
	return inf
}

func (i *FilteredNodeInformer) Lister() v1listers.NodeLister {
	return v1listers.NewNodeLister(i.Informer().GetIndexer())
}

// --- LEASE INFORMER ---
type FilteredLeaseInformer struct {
	coordinationv1.LeaseInformer
	factory *FilteredSharedInformerFactory
}

func (i *FilteredLeaseInformer) Informer() cache.SharedIndexInformer {
	// Leases in kube-node-lease often map 1:1 to nodes.
	inf := newFilteredInformer(i.LeaseInformer.Informer(), i.factory.filterKey, i.factory.filterValue, i.factory.allowMissing)
	i.factory.RegisterInformer(inf)
	return inf
}

func (i *FilteredLeaseInformer) Lister() coordinationv1listers.LeaseLister {
	return coordinationv1listers.NewLeaseLister(i.Informer().GetIndexer())
}
