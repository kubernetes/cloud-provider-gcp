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

package ane

import (
	"context"
	"fmt"
	"sync"

	gce "k8s.io/cloud-provider-gcp/providers/gce"
)

// FakeClient is an in-memory implementation of Client for testing.
type FakeClient struct {
	mu        sync.RWMutex
	Endpoints map[string]*AliasNetworkEndpoint // keyed by name
}

// NewFakeClient constructs a new FakeClient.
func NewFakeClient() *FakeClient {
	return &FakeClient{
		Endpoints: make(map[string]*AliasNetworkEndpoint),
	}
}

func (f *FakeClient) Insert(ctx context.Context, project, zone string, endpoint *AliasNetworkEndpoint) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.Endpoints[endpoint.Name]; exists {
		return "", fmt.Errorf("AliasNetworkEndpoint %q already exists", endpoint.Name)
	}

	epCopy := *endpoint
	if epCopy.Status == nil {
		epCopy.Status = &Status{State: StateActive}
	}
	// Simulate auto-assigned effective IP address if not explicitly populated
	if alias, ok := epCopy.Aliases[DefaultAliasName]; ok && alias != nil {
		if alias.EffectiveIPAddress == "" {
			if alias.IPAddress != "" {
				alias.EffectiveIPAddress = alias.IPAddress
			} else {
				alias.EffectiveIPAddress = fmt.Sprintf("10.128.0.%d", len(f.Endpoints)+1)
			}
		}
	}
	f.Endpoints[endpoint.Name] = &epCopy
	return fmt.Sprintf("op-insert-%s", endpoint.Name), nil
}

func (f *FakeClient) Delete(ctx context.Context, project, zone, name string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.Endpoints[name]; !exists {
		// Idempotent: already deleted
		return "", nil
	}
	delete(f.Endpoints, name)
	return fmt.Sprintf("op-delete-%s", name), nil
}

func (f *FakeClient) Get(ctx context.Context, project, zone, name string) (*AliasNetworkEndpoint, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.mu.RLock()
	defer f.mu.RUnlock()

	ep, exists := f.Endpoints[name]
	if !exists {
		return nil, fmt.Errorf("AliasNetworkEndpoint %q not found", name)
	}
	epCopy := *ep
	return &epCopy, nil
}

func (f *FakeClient) ListByInstance(ctx context.Context, project, zone, instanceURL string) ([]*AliasNetworkEndpoint, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.mu.RLock()
	defer f.mu.RUnlock()

	var result []*AliasNetworkEndpoint
	for _, ep := range f.Endpoints {
		if ep.Host != nil && gce.EqualResourceURLs(ep.Host.Instance, instanceURL) {
			epCopy := *ep
			result = append(result, &epCopy)
		}
	}
	return result, nil
}

func (f *FakeClient) WaitForOperation(ctx context.Context, project, zone, opName string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}
