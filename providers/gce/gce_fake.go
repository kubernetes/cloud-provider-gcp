//go:build !providerless
// +build !providerless

/*
Copyright 2018 The Kubernetes Authors.

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

package gce

import (
	"context"
	"fmt"
	"sync"

	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud"
	compute "google.golang.org/api/compute/v1"
	computepreview "google.golang.org/api/compute/v1.preview"
	option "google.golang.org/api/option"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
)

// TestClusterValues holds the config values for the fake/test gce cloud object.
type TestClusterValues struct {
	ProjectID               string
	NetworkProjectID        string
	Region                  string
	ZoneName                string
	SecondaryZoneName       string
	ClusterID               string
	ClusterName             string
	OnXPN                   bool
	Regional                bool
	NetworkURL              string
	SubnetworkURL           string
	StackType               StackType
	FirewallRulesManagement FirewallRulesManagement
}

// DefaultTestClusterValues Creates a reasonable set of default cluster values
// for generating a new test fake GCE cloud instance.
func DefaultTestClusterValues() TestClusterValues {
	return TestClusterValues{
		ProjectID:               "test-project",
		Region:                  "us-central1",
		ZoneName:                "us-central1-b",
		SecondaryZoneName:       "us-central1-c",
		ClusterID:               "test-cluster-id",
		ClusterName:             "Test-Cluster-Name",
		StackType:               clusterStackIPV4,
		FirewallRulesManagement: firewallRulesManagementEnabled,
	}
}

// Stubs ClusterID so that ClusterID.getOrInitialize() does not require calling
// gce.Initialize()
func fakeClusterID(clusterID string) ClusterID {
	return ClusterID{
		clusterID: &clusterID,
		store: cache.NewStore(func(obj interface{}) (string, error) {
			return "", nil
		}),
	}
}

// NewFakeGCECloud constructs a fake GCE Cloud from the cluster values.
func NewFakeGCECloud(vals TestClusterValues) *Cloud {
	service, err := compute.NewService(context.Background(), option.WithoutAuthentication())
	if err != nil {
		panic(err)
	}
	servicePreview, err := computepreview.NewService(context.Background(), option.WithoutAuthentication())
	if err != nil {
		panic(err)
	}
	netProjID := vals.NetworkProjectID
	if netProjID == "" {
		netProjID = vals.ProjectID
	}
	gce := &Cloud{
		region:                  vals.Region,
		service:                 service,
		servicePreview:          servicePreview,
		managedZones:            []string{vals.ZoneName},
		localZone:               vals.ZoneName,
		projectID:               vals.ProjectID,
		networkProjectID:        netProjID,
		ClusterID:               fakeClusterID(vals.ClusterID),
		onXPN:                   vals.OnXPN,
		metricsCollector:        newLoadBalancerMetrics(),
		projectsBasePath:        getProjectsBasePath(service.BasePath),
		regional:                vals.Regional,
		dynamicZones:            vals.Regional,
		networkURL:              vals.NetworkURL,
		unsafeSubnetworkURL:     vals.SubnetworkURL,
		stackType:               vals.StackType,
		nodeZones:               map[string]sets.String{},
		firewallRulesManagement: vals.FirewallRulesManagement,
	}
	c := cloud.NewMockGCE(&gceProjectRouter{gce})
	gce.c = c
	gce.aneService = newFakeANEService()
	return gce
}

// UpdateFakeGCECloud updates the fake GCE cloud with the specified values. Currently only the onXPN value is updated.
func UpdateFakeGCECloud(g *Cloud, vals TestClusterValues) {
	g.onXPN = vals.OnXPN
}

// SetFakeStackType updates the fake GCE cloud with the specified stack type.
func SetFakeStackType(g *Cloud, stackType StackType) {
	g.stackType = stackType
}

type fakeANEService struct {
	mu        sync.RWMutex
	endpoints map[string]*computepreview.AliasNetworkEndpoint
	nextIP    int
}

func newFakeANEService() *fakeANEService {
	return &fakeANEService{
		endpoints: make(map[string]*computepreview.AliasNetworkEndpoint),
	}
}

func (f *fakeANEService) CreateAliasNetworkEndpoint(ctx context.Context, project, zone string, endpoint *computepreview.AliasNetworkEndpoint) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.endpoints[endpoint.Name]; exists {
		return fmt.Errorf("AliasNetworkEndpoint %q already exists", endpoint.Name)
	}

	epCopy := *endpoint
	if len(endpoint.Aliases) > 0 {
		epCopy.Aliases = make(map[string]computepreview.AliasNetworkEndpointAlias, len(endpoint.Aliases))
		for k, v := range endpoint.Aliases {
			epCopy.Aliases[k] = v
		}
	}
	if epCopy.Status == nil {
		epCopy.Status = &computepreview.AliasNetworkEndpointStatus{State: ANEStateActive}
		for k, alias := range epCopy.Aliases {
			if alias.EffectiveIpAddress == "" {
				if alias.IpAddress != "" {
					alias.EffectiveIpAddress = alias.IpAddress
				} else {
					f.nextIP++
					alias.EffectiveIpAddress = fmt.Sprintf("10.128.0.%d", f.nextIP)
				}
				epCopy.Aliases[k] = alias
			}
		}
	}
	f.endpoints[endpoint.Name] = &epCopy
	return nil
}

func (f *fakeANEService) DeleteAliasNetworkEndpoint(ctx context.Context, project, zone, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	delete(f.endpoints, name)
	return nil
}

func (f *fakeANEService) ListAliasNetworkEndpoints(ctx context.Context, project, zone, instanceURL string) ([]*computepreview.AliasNetworkEndpoint, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.mu.RLock()
	defer f.mu.RUnlock()

	var result []*computepreview.AliasNetworkEndpoint
	for _, ep := range f.endpoints {
		if ep.Host != nil && EqualResourceURLs(ep.Host.Instance, instanceURL) {
			epCopy := *ep
			result = append(result, &epCopy)
		}
	}
	return result, nil
}
