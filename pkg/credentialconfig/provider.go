/*
Copyright 2014 The Kubernetes Authors.

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

package credentialconfig

import (
	"reflect"
	"sync"
	"time"

	"k8s.io/klog/v2"
)

// DockerConfigProvider is the interface that registered extensions implement
// to materialize 'dockercfg' credentials.
type DockerConfigProvider interface {
	// Enabled returns true if the config provider is enabled.
	// Implementations can be blocking - e.g. metadata server unavailable.
	Enabled() bool
	// Provide returns docker configuration.
	// Implementations can be blocking - e.g. metadata server unavailable.
	// The image is passed in as context in the event that the
	// implementation depends on information in the image name to return
	// credentials; implementations are safe to ignore the image.
	Provide(image string) DockerConfig
}

type cacheEntry struct {
	config     DockerConfig
	expiration time.Time
}

// CachingDockerConfigProvider implements DockerConfigProvider by composing
// with another DockerConfigProvider and caching the DockerConfig it provides
// for a pre-specified lifetime.
type CachingDockerConfigProvider struct {
	Provider DockerConfigProvider
	Lifetime time.Duration

	// ShouldCache is an optional function that returns true if the specific config should be cached.
	// If nil, all configs are treated as cacheable.
	ShouldCache func(DockerConfig) bool

	// cache fields
	cache map[string]cacheEntry
	mu    sync.Mutex
}

// Enabled implements dockerConfigProvider
func (d *CachingDockerConfigProvider) Enabled() bool {
	return d.Provider.Enabled()
}

func deepCopyDockerConfig(cfg DockerConfig) DockerConfig {
	if cfg == nil {
		return nil
	}
	res := make(DockerConfig, len(cfg))
	for k, v := range cfg {
		res[k] = v
	}
	return res
}

// Provide implements dockerConfigProvider
func (d *CachingDockerConfigProvider) Provide(image string) DockerConfig {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.cache == nil {
		d.cache = make(map[string]cacheEntry)
	}

	// If the cache entry exists and hasn't expired, return a deep copy of our cached config
	if entry, ok := d.cache[image]; ok && time.Now().Before(entry.expiration) {
		return deepCopyDockerConfig(entry.config)
	}

	klog.V(2).Infof("Refreshing cache for provider: %v", reflect.TypeOf(d.Provider).String())
	config := d.Provider.Provide(image)
	if d.ShouldCache == nil || d.ShouldCache(config) {
		d.cache[image] = cacheEntry{
			config:     deepCopyDockerConfig(config),
			expiration: time.Now().Add(d.Lifetime),
		}
	}
	return deepCopyDockerConfig(config)
}
