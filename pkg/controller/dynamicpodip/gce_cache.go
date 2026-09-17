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
	"container/list"
	"context"
	"sync"
	"time"

	computebeta "google.golang.org/api/compute/v0.beta"
	"k8s.io/utils/clock"
)

const (
	// DefaultCacheMaxEntries is the default maximum number of nodes retained by a GCECache.
	// Once the cache is full, adding a new node evicts the least recently used one.
	DefaultCacheMaxEntries = 1 << 16

	// DefaultCacheMaxAge is the default maximum age of a cached observation. An entry whose
	// observation has not been refreshed within this window is evicted, so that nodes which
	// have been deleted (or are simply no longer reconciled) do not accumulate forever.
	DefaultCacheMaxAge = 10 * time.Minute
)

// networkInterface is a controller-internal, lightweight representation of a GCE network interface.
type networkInterface struct {
	Name          string
	Network       string
	AliasIPRanges []string
}

// toNetworkInterfaces converts a slice of GCE API computebeta.NetworkInterface objects to controller-internal networkInterface objects.
func toNetworkInterfaces(gceIfaces []*computebeta.NetworkInterface) []*networkInterface {
	if gceIfaces == nil {
		return nil
	}
	res := make([]*networkInterface, len(gceIfaces))
	for i, iface := range gceIfaces {
		if iface == nil {
			continue
		}
		ni := &networkInterface{
			Name:    iface.Name,
			Network: iface.Network,
		}
		for _, r := range iface.AliasIpRanges {
			if r != nil && r.IpCidrRange != "" {
				ni.AliasIPRanges = append(ni.AliasIPRanges, r.IpCidrRange)
			}
		}
		res[i] = ni
	}
	return res
}

// deepCopyInterfaces performs a deep copy of internal network interfaces to prevent data races.
func deepCopyInterfaces(ifaces []*networkInterface) []*networkInterface {
	if ifaces == nil {
		return nil
	}
	copy := make([]*networkInterface, len(ifaces))
	for i, ni := range ifaces {
		if ni == nil {
			continue
		}
		niCopy := &networkInterface{
			Name:    ni.Name,
			Network: ni.Network,
		}
		if ni.AliasIPRanges != nil {
			niCopy.AliasIPRanges = append([]string(nil), ni.AliasIPRanges...)
		}
		copy[i] = niCopy
	}
	return copy
}

// GCEInstanceLoader is a functional dependency injected into the cache to fetch fresh data from GCE.
type GCEInstanceLoader func(ctx context.Context, providerID string) ([]*networkInterface, error)

// CachedInstance represents a cached view of a single GCE instance's network interfaces, protected by its own mutex.
type CachedInstance struct {
	mu          sync.Mutex
	interfaces  []*networkInterface
	lastUpdated time.Time
}

// cacheEntry is the bookkeeping record for a single node. It is owned by GCECache.mapLock; the
// cached observation itself lives in inst and is guarded by inst.mu.
type cacheEntry struct {
	nodeName string
	inst     *CachedInstance

	// expiresAt is when this entry's observation ages out. It is always clock.Now()+maxAge as
	// computed under mapLock, which is what keeps ageList sorted.
	expiresAt time.Time

	// lruElem is this entry's element in GCECache.lruList and ageElem its element in
	// GCECache.ageList. Both carry the *cacheEntry as their value.
	lruElem *list.Element
	ageElem *list.Element
}

// GCECache manages thread-safe, concurrent timed caching of GCE instance states using per-node locking.
//
// The cache is bounded both by observation age (maxAge) and by entry count (maxEntries), so that it
// does not grow without bound over the lifetime of a cluster as nodes are created and deleted.
type GCECache struct {
	// mapLock guards entries, lruList and ageList.
	//
	// Lock ordering: mapLock is never held while acquiring a CachedInstance's mutex, so it is
	// safe to acquire mapLock while holding one.
	mapLock sync.Mutex
	// entries maps node name to its bookkeeping record.
	entries map[string]*cacheEntry
	// lruList orders entries by recency of use, most recently used at the front. Capacity
	// eviction pops from the back.
	lruList *list.List
	// ageList orders entries by expiry, soonest to expire at the front. Because maxAge is
	// constant and the clock is monotonic, appending to the back whenever an entry's expiry is
	// recomputed keeps the list sorted, which is what lets ageCleanUp walk only the expired
	// prefix instead of scanning every entry.
	ageList *list.List

	loader     GCEInstanceLoader
	ttl        time.Duration
	maxAge     time.Duration
	maxEntries int
	clock      clock.Clock
}

// NewGCECache constructs a new GCE loading cache with the default eviction limits.
func NewGCECache(loader GCEInstanceLoader, ttl time.Duration, clock clock.Clock) *GCECache {
	return NewGCECacheWithLimits(loader, ttl, DefaultCacheMaxAge, DefaultCacheMaxEntries, clock)
}

// NewGCECacheWithLimits constructs a new GCE loading cache with explicit eviction limits.
//
// ttl controls how long an observation may be reused before it is refreshed from GCE, whereas
// maxAge controls how long an entry is retained at all. maxEntries caps the number of nodes
// tracked; adding a node beyond the cap evicts the least recently used one. Non-positive values
// for maxAge or maxEntries fall back to the defaults.
func NewGCECacheWithLimits(loader GCEInstanceLoader, ttl, maxAge time.Duration, maxEntries int, clk clock.Clock) *GCECache {
	if maxAge <= 0 {
		maxAge = DefaultCacheMaxAge
	}
	if maxEntries <= 0 {
		maxEntries = DefaultCacheMaxEntries
	}
	return &GCECache{
		entries:    make(map[string]*cacheEntry),
		lruList:    list.New(),
		ageList:    list.New(),
		loader:     loader,
		ttl:        ttl,
		maxAge:     maxAge,
		maxEntries: maxEntries,
		clock:      clk,
	}
}

// ageCleanUp evicts every entry whose observation has aged out.
//
// Because ageList is sorted by expiry, this walks only the expired prefix and stops at the first
// live entry, so the common case where nothing has aged out costs a single comparison. This is what
// lets it run on every lookup, which in turn means entries for deleted nodes are reclaimed as soon
// as any other node is reconciled rather than lingering until the entry cap is reached.
//
// The caller must hold mapLock.
func (c *GCECache) ageCleanUp(now time.Time) {
	for {
		front := c.ageList.Front()
		if front == nil {
			return
		}
		entry := front.Value.(*cacheEntry)
		if now.Before(entry.expiresAt) {
			return
		}
		c.removeEntry(entry)
	}
}

// removeEntry drops an entry from the map and from both ordering lists.
//
// The caller must hold mapLock.
func (c *GCECache) removeEntry(entry *cacheEntry) {
	c.lruList.Remove(entry.lruElem)
	c.ageList.Remove(entry.ageElem)
	delete(c.entries, entry.nodeName)
}

// addEntry inserts a new entry, first evicting least recently used entries if the cache is full.
//
// The caller must hold mapLock.
func (c *GCECache) addEntry(nodeName string, inst *CachedInstance, now time.Time) {
	for c.lruList.Len() >= c.maxEntries {
		back := c.lruList.Back()
		if back == nil {
			break
		}
		c.removeEntry(back.Value.(*cacheEntry))
	}

	entry := &cacheEntry{
		nodeName:  nodeName,
		inst:      inst,
		expiresAt: now.Add(c.maxAge),
	}
	entry.lruElem = c.lruList.PushFront(entry)
	entry.ageElem = c.ageList.PushBack(entry)
	c.entries[nodeName] = entry
}

// getOrCreateInstance retrieves or initializes the CachedInstance for a node, marking it as the most
// recently used entry. Aged-out entries are reclaimed first.
func (c *GCECache) getOrCreateInstance(nodeName string) *CachedInstance {
	c.mapLock.Lock()
	defer c.mapLock.Unlock()

	now := c.clock.Now()
	c.ageCleanUp(now)

	if entry, ok := c.entries[nodeName]; ok {
		c.lruList.MoveToFront(entry.lruElem)
		return entry.inst
	}

	inst := &CachedInstance{}
	c.addEntry(nodeName, inst, now)
	return inst
}

// touch resets the eviction deadline for nodeName, keeping the entry alive for another maxAge now
// that its observation has been refreshed. This makes maxAge measure the age of the observation
// rather than the age of the cache entry.
//
// touch only maintains the expiry ordering; recency of use is maintained by getOrCreateInstance,
// which has already fronted this entry earlier in the same get() call.
//
// The current time is read under mapLock rather than passed in, so that the order in which entries
// are appended to ageList always matches the order of their expiry times.
func (c *GCECache) touch(nodeName string, inst *CachedInstance) {
	c.mapLock.Lock()
	defer c.mapLock.Unlock()

	now := c.clock.Now()

	entry, ok := c.entries[nodeName]
	if !ok {
		// The entry aged out or was evicted for capacity while the load was in flight.
		// Reinstate it so the work is not wasted.
		c.addEntry(nodeName, inst, now)
		return
	}
	if entry.inst != inst {
		// A concurrent reconcile of the same node already replaced this entry. Leave the
		// newer entry alone; our caller still gets the observation it just loaded.
		return
	}

	entry.expiresAt = now.Add(c.maxAge)
	c.ageList.MoveToBack(entry.ageElem)
}

// Get retrieves the cached network interfaces for the node.
// If the cache is stale or missing, it calls the GCE loader holding ONLY the node-specific lock.
func (c *GCECache) Get(ctx context.Context, nodeName string, providerID string) ([]*networkInterface, error) {
	return c.get(ctx, nodeName, providerID, false)
}

// ForceGet bypasses the TTL check, forces a fresh load from GCE, updates the cache, and returns the state.
func (c *GCECache) ForceGet(ctx context.Context, nodeName string, providerID string) ([]*networkInterface, error) {
	return c.get(ctx, nodeName, providerID, true)
}

func (c *GCECache) get(ctx context.Context, nodeName string, providerID string, force bool) ([]*networkInterface, error) {
	inst := c.getOrCreateInstance(nodeName)

	inst.mu.Lock()
	defer inst.mu.Unlock()

	now := c.clock.Now()
	if force || inst.lastUpdated.IsZero() || now.Sub(inst.lastUpdated) > c.ttl {
		ifaces, err := c.loader(ctx, providerID)
		if err != nil {
			return nil, err
		}
		inst.interfaces = deepCopyInterfaces(ifaces)
		inst.lastUpdated = now
		c.touch(nodeName, inst)
	}

	return deepCopyInterfaces(inst.interfaces), nil
}
