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
	"context"
	"sync"
	"time"

	computebeta "google.golang.org/api/compute/v0.beta"
	"k8s.io/utils/clock"
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
	mu          sync.Mutex // Guards this specific node's cached interfaces and timestamp
	interfaces  []*networkInterface
	lastUpdated time.Time
}

// GCECache manages thread-safe, concurrent timed caching of GCE instance states using per-node locking.
type GCECache struct {
	mapLock   sync.RWMutex // Guards the map structure itself
	instances map[string]*CachedInstance
	loader    GCEInstanceLoader
	ttl       time.Duration
	clock     clock.Clock
}

// NewGCECache constructs a new GCE loading cache.
func NewGCECache(loader GCEInstanceLoader, ttl time.Duration, clock clock.Clock) *GCECache {
	return &GCECache{
		instances: make(map[string]*CachedInstance),
		loader:    loader,
		ttl:       ttl,
		clock:     clock,
	}
}

// getOrCreateInstance retrieves or initializes the CachedInstance pointer for a node under the global map lock.
func (c *GCECache) getOrCreateInstance(nodeName string) *CachedInstance {
	c.mapLock.Lock()
	defer c.mapLock.Unlock()

	inst, ok := c.instances[nodeName]
	if !ok {
		inst = &CachedInstance{}
		c.instances[nodeName] = inst
	}
	return inst
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

	// Lock only this specific node's state. Other nodes can be processed concurrently.
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
	}

	return deepCopyInterfaces(inst.interfaces), nil
}
