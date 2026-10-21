/*
Copyright 2026 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package dynamicpodip

import (
	"sort"
	"strings"
	"sync"

	networkv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/network/v1"
	networkinformer "github.com/GoogleCloudPlatform/gke-networking-api/client/network/informers/externalversions/network/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

const gnpKind = "GKENetworkParamSet"

// NetworkParams is the resolved view of a Kubernetes Network and the
// GKENetworkParamSet it references, reduced to what the controllers need to
// match it against GCE interfaces and alias IP ranges.
type NetworkParams struct {
	NetworkName       string
	GNPName           string
	VPC               string
	VPCSubnet         string
	NetworkAttachment string
	PodIPv4RangeNames []string
	// Ready reports whether the Network carries Ready=True (the default
	// network is always considered Ready). A Network that is indexed but not
	// Ready still *claims* its ranges, so that nothing else can absorb them,
	// but the controllers neither allocate on it nor publish its ranges.
	Ready bool
}

type vpcSubnetRangeKey struct {
	vpc       string
	subnet    string
	rangeName string
}

// NetworkResolver maps between Kubernetes Networks (via their
// GKENetworkParamSet) and GCE network interfaces / alias IP ranges.
//
// It keeps a small in-memory index that is rebuilt from informer events, so
// that the lookups the controllers perform while reconciling a
// NodeNetworkConfig are O(1) map reads with no lister scans or API calls. The
// number of Networks in a cluster is capped and changes rarely, so rebuilding
// the whole index on every event is cheap and keeps the code simple.
//
// Matching mirrors the multi-network CIDR allocator in nodeipam/ipam: a
// Network matches an interface on NetworkAttachment, or on (VPC, VPCSubnet)
// plus the alias range's SubnetworkRangeName being one of the GNP's
// PodIPv4Ranges. Readiness is recorded per Network rather than used as an
// index filter: see NetworkParams.Ready.
type NetworkResolver struct {
	mu sync.RWMutex

	// synced reports whether the backing informers have delivered their
	// initial events to this resolver's handlers.
	synced []cache.InformerSynced

	networks map[string]*networkv1.Network
	gnps     map[string]*networkv1.GKENetworkParamSet

	// Derived lookup tables, rebuilt whenever networks or gnps change.
	byNetworkName    map[string]NetworkParams
	byAttachment     map[string]string
	byVPCSubnetRange map[vpcSubnetRangeKey]string

	// onChange is invoked, outside the lock, after every index rebuild. It is
	// fixed at construction and never mutated, so it needs no locking.
	onChange []func()
}

// NewNetworkResolver creates a NetworkResolver fed by the given informers.
// Either informer may be nil, in which case the resolver can still be
// populated directly through UpsertNetwork and UpsertGNP (used by tests).
//
// Each onChange function is called after any Network or GKENetworkParamSet
// change has been folded into the index. They run synchronously on the
// informer goroutine, after the resolver's lock has been released, so they may
// call back into the resolver but must return quickly. This is how the status
// controller learns that a Network has become Ready, stopped being Ready, or
// disappeared -- events that change which nodes should have a
// NodeNetworkConfig but are invisible to its Node watch.
func NewNetworkResolver(
	networkInformer networkinformer.NetworkInformer,
	gnpInformer networkinformer.GKENetworkParamSetInformer,
	onChange ...func(),
) *NetworkResolver {
	r := &NetworkResolver{
		networks:         make(map[string]*networkv1.Network),
		gnps:             make(map[string]*networkv1.GKENetworkParamSet),
		byNetworkName:    make(map[string]NetworkParams),
		byAttachment:     make(map[string]string),
		byVPCSubnetRange: make(map[vpcSubnetRangeKey]string),
		onChange:         append([]func(){}, onChange...),
	}

	if networkInformer != nil {
		reg, err := networkInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    r.handleNetworkAddOrUpdate,
			UpdateFunc: func(_, newObj interface{}) { r.handleNetworkAddOrUpdate(newObj) },
			DeleteFunc: r.handleNetworkDelete,
		})
		if err != nil {
			klog.Errorf("Failed to register Network event handler: %v", err)
		} else {
			// The registration's HasSynced covers delivery of the initial
			// list to this handler, not just population of the store, so
			// waiting on it guarantees the index is complete.
			r.synced = append(r.synced, reg.HasSynced)
		}
	}

	if gnpInformer != nil {
		reg, err := gnpInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    r.handleGNPAddOrUpdate,
			UpdateFunc: func(_, newObj interface{}) { r.handleGNPAddOrUpdate(newObj) },
			DeleteFunc: r.handleGNPDelete,
		})
		if err != nil {
			klog.Errorf("Failed to register GKENetworkParamSet event handler: %v", err)
		} else {
			r.synced = append(r.synced, reg.HasSynced)
		}
	}

	return r
}

// Synced returns the InformerSynced functions a caller must wait on before
// relying on the resolver. It is safe to call on a nil resolver.
func (r *NetworkResolver) Synced() []cache.InformerSynced {
	if r == nil {
		return nil
	}
	return r.synced
}

func (r *NetworkResolver) handleNetworkAddOrUpdate(obj interface{}) {
	if nw, ok := obj.(*networkv1.Network); ok && nw != nil {
		r.UpsertNetwork(nw)
	}
}

func (r *NetworkResolver) handleNetworkDelete(obj interface{}) {
	if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		obj = tombstone.Obj
	}
	if nw, ok := obj.(*networkv1.Network); ok && nw != nil {
		r.DeleteNetwork(nw.Name)
	}
}

func (r *NetworkResolver) handleGNPAddOrUpdate(obj interface{}) {
	if gnp, ok := obj.(*networkv1.GKENetworkParamSet); ok && gnp != nil {
		r.UpsertGNP(gnp)
	}
}

func (r *NetworkResolver) handleGNPDelete(obj interface{}) {
	if tombstone, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		obj = tombstone.Obj
	}
	if gnp, ok := obj.(*networkv1.GKENetworkParamSet); ok && gnp != nil {
		r.DeleteGNP(gnp.Name)
	}
}

// mutate applies fn to the raw Network/GNP maps under the write lock, rebuilds
// the index, and then -- with the lock released -- runs the onChange
// functions. It is the single path through which the index changes, so the
// "notify outside the lock" invariant lives in exactly one place.
func (r *NetworkResolver) mutate(fn func()) {
	func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		fn()
		r.rebuildIndexLocked()
	}()
	for _, notify := range r.onChange {
		notify()
	}
}

// UpsertNetwork adds or replaces a Network in the index.
func (r *NetworkResolver) UpsertNetwork(nw *networkv1.Network) {
	if r == nil || nw == nil {
		return
	}
	r.mutate(func() { r.networks[nw.Name] = nw.DeepCopy() })
}

// DeleteNetwork removes a Network from the index.
func (r *NetworkResolver) DeleteNetwork(name string) {
	if r == nil {
		return
	}
	r.mutate(func() { delete(r.networks, name) })
}

// UpsertGNP adds or replaces a GKENetworkParamSet in the index.
func (r *NetworkResolver) UpsertGNP(gnp *networkv1.GKENetworkParamSet) {
	if r == nil || gnp == nil {
		return
	}
	r.mutate(func() { r.gnps[gnp.Name] = gnp.DeepCopy() })
}

// DeleteGNP removes a GKENetworkParamSet from the index.
func (r *NetworkResolver) DeleteGNP(name string) {
	if r == nil {
		return
	}
	r.mutate(func() { delete(r.gnps, name) })
}

// rebuildIndexLocked reconstructs the lookup tables from networks and gnps.
// The caller must hold r.mu for writing.
//
// Only L3 Networks whose ParametersRef names an existing, non-device-mode GNP
// are indexed; everything else is invisible to the controllers. Networks are
// visited in name order so that, should two Networks ever claim the same
// (VPC, subnet, range) or attachment, the winner is deterministic.
func (r *NetworkResolver) rebuildIndexLocked() {
	byNetworkName := make(map[string]NetworkParams, len(r.networks))
	byAttachment := make(map[string]string)
	byVPCSubnetRange := make(map[vpcSubnetRangeKey]string)

	names := make([]string, 0, len(r.networks))
	for name := range r.networks {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		nw := r.networks[name]
		if nw.Spec.Type == networkv1.DeviceNetworkType {
			continue
		}
		if nw.Spec.ParametersRef == nil || !strings.EqualFold(nw.Spec.ParametersRef.Kind, gnpKind) {
			continue
		}
		gnp := r.gnps[nw.Spec.ParametersRef.Name]
		if gnp == nil || gnp.Spec.DeviceMode != "" {
			continue
		}

		var rangeNames []string
		if gnp.Spec.PodIPv4Ranges != nil {
			rangeNames = append([]string(nil), gnp.Spec.PodIPv4Ranges.RangeNames...)
		}

		// The default network is treated as Ready unconditionally, as it is
		// in the multi-network CIDR allocator: it is in use before NCM ever
		// marks it Ready, and in IPv6-only clusters it never is.
		ready := nw.Name == networkv1.DefaultPodNetworkName ||
			apimeta.IsStatusConditionTrue(nw.Status.Conditions, string(networkv1.NetworkConditionStatusReady))

		byNetworkName[nw.Name] = NetworkParams{
			NetworkName:       nw.Name,
			GNPName:           gnp.Name,
			VPC:               gnp.Spec.VPC,
			VPCSubnet:         gnp.Spec.VPCSubnet,
			NetworkAttachment: gnp.Spec.NetworkAttachment,
			PodIPv4RangeNames: rangeNames,
			Ready:             ready,
		}

		if att := canonicalSubnetwork(gnp.Spec.NetworkAttachment); att != "" {
			if prev, dup := byAttachment[att]; dup {
				klog.Warningf("Networks %q and %q both reference network attachment %q; using %q", prev, nw.Name, att, prev)
				continue
			}
			byAttachment[att] = nw.Name
			continue
		}

		vpc := canonicalSubnetwork(gnp.Spec.VPC)
		subnet := canonicalSubnetwork(gnp.Spec.VPCSubnet)
		if vpc == "" || subnet == "" {
			continue
		}
		for _, rn := range rangeNames {
			key := vpcSubnetRangeKey{vpc: vpc, subnet: subnet, rangeName: strings.TrimSpace(rn)}
			if key.rangeName == "" {
				continue
			}
			if prev, dup := byVPCSubnetRange[key]; dup {
				klog.Warningf("Networks %q and %q both claim secondary range %q in subnet %q; using %q", prev, nw.Name, key.rangeName, subnet, prev)
				continue
			}
			byVPCSubnetRange[key] = nw.Name
		}
	}

	r.byNetworkName = byNetworkName
	r.byAttachment = byAttachment
	r.byVPCSubnetRange = byVPCSubnetRange
}

// LookupNetworkParams returns the resolved parameters for a Kubernetes
// Network name. It is safe to call on a nil resolver, which never matches.
func (r *NetworkResolver) LookupNetworkParams(netName string) (NetworkParams, bool) {
	if r == nil {
		return NetworkParams{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	params, ok := r.byNetworkName[netName]
	if !ok {
		return NetworkParams{}, false
	}
	params.PodIPv4RangeNames = append([]string(nil), params.PodIPv4RangeNames...)
	return params, true
}

// ReadyAdditionalNetworks returns the resolved parameters of every indexed
// Network that is Ready, excluding the default network, in name order.
//
// "Additional" matters: the default Network CR exists in every multi-network
// cluster (and is always considered Ready), so including it would make "does
// this node participate in any Ready Network" trivially true. It is safe to
// call on a nil resolver, which returns nothing.
func (r *NetworkResolver) ReadyAdditionalNetworks() []NetworkParams {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.byNetworkName))
	for name, params := range r.byNetworkName {
		if name == networkv1.DefaultPodNetworkName || !params.Ready {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]NetworkParams, 0, len(names))
	for _, name := range names {
		params := r.byNetworkName[name]
		params.PodIPv4RangeNames = append([]string(nil), params.PodIPv4RangeNames...)
		out = append(out, params)
	}
	return out
}

// networkByAttachment returns the Network whose GNP references the given
// network attachment (URL or short name), and whether it is Ready.
func (r *NetworkResolver) networkByAttachment(attachment string) (name string, ready, ok bool) {
	if r == nil {
		return "", false, false
	}
	att := canonicalSubnetwork(attachment)
	if att == "" {
		return "", false, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	name, ok = r.byAttachment[att]
	if !ok {
		return "", false, false
	}
	return name, r.byNetworkName[name].Ready, true
}

// networkByVPCSubnetRange returns the Network whose GNP lists rangeName as a
// pod range in the given VPC and subnetwork (URLs or short names), and whether
// it is Ready.
func (r *NetworkResolver) networkByVPCSubnetRange(vpc, subnet, rangeName string) (name string, ready, ok bool) {
	if r == nil {
		return "", false, false
	}
	key := vpcSubnetRangeKey{
		vpc:       canonicalSubnetwork(vpc),
		subnet:    canonicalSubnetwork(subnet),
		rangeName: strings.TrimSpace(rangeName),
	}
	if key.vpc == "" || key.subnet == "" || key.rangeName == "" {
		return "", false, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	name, ok = r.byVPCSubnetRange[key]
	if !ok {
		return "", false, false
	}
	return name, r.byNetworkName[name].Ready, true
}
