package main

import (
	"testing"

	cloudprovider "k8s.io/cloud-provider"
	nodeipamconfig "k8s.io/cloud-provider-gcp/pkg/controller/nodeipam/config"
	cloudcontrollerconfig "k8s.io/cloud-provider/app/config"
	"k8s.io/cloud-provider/config"
	genericcontrollermanager "k8s.io/controller-manager/app"
)

type fakeCloudProvider struct{}

// Implements cloudprovider.Interface.
var _ cloudprovider.Interface = &fakeCloudProvider{}

func (f *fakeCloudProvider) Initialize(clientBuilder cloudprovider.ControllerClientBuilder, stop <-chan struct{}) {
}

func (f *fakeCloudProvider) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	return nil, false
}

func (f *fakeCloudProvider) Instances() (cloudprovider.Instances, bool) {
	return nil, false
}

func (f *fakeCloudProvider) InstancesV2() (cloudprovider.InstancesV2, bool) {
	return nil, false
}

func (f *fakeCloudProvider) Zones() (cloudprovider.Zones, bool) {
	return nil, false
}

func (f *fakeCloudProvider) Clusters() (cloudprovider.Clusters, bool) {
	return nil, false
}

func (f *fakeCloudProvider) Routes() (cloudprovider.Routes, bool) {
	return nil, false
}

func (f *fakeCloudProvider) ProviderName() string {
	return "fake"
}

func (f *fakeCloudProvider) HasClusterID() bool {
	return false
}

func TestStartNodeIpamController(t *testing.T) {
	testCases := []struct {
		desc           string
		ccmConfig      *cloudcontrollerconfig.Config
		nodeIPAMConfig nodeipamconfig.NodeIPAMControllerConfiguration
		wantErr        bool
	}{
		{
			desc: "Allocate node CIDRs disabled",
			ccmConfig: &cloudcontrollerconfig.Config{
				ComponentConfig: config.CloudControllerManagerConfiguration{
					KubeCloudShared: config.KubeCloudSharedConfiguration{
						AllocateNodeCIDRs: false,
					},
				},
			},
			wantErr: true,
		},
		{
			desc: "Unparseable cluster CIDRs",
			ccmConfig: &cloudcontrollerconfig.Config{
				ComponentConfig: config.CloudControllerManagerConfiguration{
					KubeCloudShared: config.KubeCloudSharedConfiguration{
						AllocateNodeCIDRs: true,
						ClusterCIDR:       "invalid",
					},
				},
			},
			wantErr: true,
		},
		{
			desc: "Multiple same stack type cluster CIDRs - ipv4",
			ccmConfig: &cloudcontrollerconfig.Config{
				ComponentConfig: config.CloudControllerManagerConfiguration{
					KubeCloudShared: config.KubeCloudSharedConfiguration{
						AllocateNodeCIDRs: true,
						ClusterCIDR:       "10.0.0.0/16,10.1.0.0/16",
					},
				},
			},
			wantErr: true,
		},
		{
			desc: "Multiple same stack type cluster CIDRs - ipv6",
			ccmConfig: &cloudcontrollerconfig.Config{
				ComponentConfig: config.CloudControllerManagerConfiguration{
					KubeCloudShared: config.KubeCloudSharedConfiguration{
						AllocateNodeCIDRs: true,
						ClusterCIDR:       "2001:db8::/112,2001:db9::/112",
					},
				},
			},
			wantErr: true,
		},
		{
			desc: "More than 2 cluster CIDRs",
			ccmConfig: &cloudcontrollerconfig.Config{
				ComponentConfig: config.CloudControllerManagerConfiguration{
					KubeCloudShared: config.KubeCloudSharedConfiguration{
						AllocateNodeCIDRs: true,
						ClusterCIDR:       "10.0.0.0/16,10.1.0.0/16,10.2.0.0/16",
					},
				},
			},
			wantErr: true,
		},
		{
			desc: "Primary and secondary service CIDR same stack type - ipv4",
			ccmConfig: &cloudcontrollerconfig.Config{
				ComponentConfig: config.CloudControllerManagerConfiguration{
					KubeCloudShared: config.KubeCloudSharedConfiguration{
						AllocateNodeCIDRs: true,
						ClusterCIDR:       "10.0.0.0/16",
					},
				},
			},
			nodeIPAMConfig: nodeipamconfig.NodeIPAMControllerConfiguration{
				ServiceCIDR:          "10.0.0.0/16",
				SecondaryServiceCIDR: "10.1.0.0/16",
			},
			wantErr: true,
		},
		{
			desc: "Primary and secondary service CIDR same stack type - ipv6",
			ccmConfig: &cloudcontrollerconfig.Config{
				ComponentConfig: config.CloudControllerManagerConfiguration{
					KubeCloudShared: config.KubeCloudSharedConfiguration{
						AllocateNodeCIDRs: true,
						ClusterCIDR:       "10.0.0.0/16",
					},
				},
			},
			nodeIPAMConfig: nodeipamconfig.NodeIPAMControllerConfiguration{
				ServiceCIDR:          "2001:db8::/112",
				SecondaryServiceCIDR: "2001:db9::/112",
			},
			wantErr: true,
		},
		{
			desc: "NodeCIDRMaskSize used with a dual stack cluster",
			ccmConfig: &cloudcontrollerconfig.Config{
				ComponentConfig: config.CloudControllerManagerConfiguration{
					KubeCloudShared: config.KubeCloudSharedConfiguration{
						AllocateNodeCIDRs: true,
						ClusterCIDR:       "10.0.0.0/16,2001:aa::/112",
					},
				},
			},
			nodeIPAMConfig: nodeipamconfig.NodeIPAMControllerConfiguration{
				NodeCIDRMaskSize: 10,
			},
			wantErr: true,
		},
		{
			desc: "NodeCIDRMaskSize and NodeCIDRMaskSizeIPv4 used together",
			ccmConfig: &cloudcontrollerconfig.Config{
				ComponentConfig: config.CloudControllerManagerConfiguration{
					KubeCloudShared: config.KubeCloudSharedConfiguration{
						AllocateNodeCIDRs: true,
						ClusterCIDR:       "10.0.0.0/16",
					},
				},
			},
			nodeIPAMConfig: nodeipamconfig.NodeIPAMControllerConfiguration{
				NodeCIDRMaskSize:     10,
				NodeCIDRMaskSizeIPv4: 4,
			},
			wantErr: true,
		},
		{
			desc: "NodeCIDRMaskSize and NodeCIDRMaskSizeIPv6 used together",
			ccmConfig: &cloudcontrollerconfig.Config{
				ComponentConfig: config.CloudControllerManagerConfiguration{
					KubeCloudShared: config.KubeCloudSharedConfiguration{
						AllocateNodeCIDRs: true,
						ClusterCIDR:       "10.0.0.0/16",
					},
				},
			},
			nodeIPAMConfig: nodeipamconfig.NodeIPAMControllerConfiguration{
				NodeCIDRMaskSize:     10,
				NodeCIDRMaskSizeIPv6: 4,
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			ctx := genericcontrollermanager.ControllerContext{}
			_, _, err := startNodeIpamController(tc.ccmConfig.Complete(), tc.nodeIPAMConfig, ctx, &fakeCloudProvider{})

			if err == nil && tc.wantErr {
				t.Fatalf("startNodeIpamController succeeded, want error")
			}
			if err != nil && !tc.wantErr {
				t.Fatalf("startNodeIpamController(): %v", err)
			}
		})
	}
}
