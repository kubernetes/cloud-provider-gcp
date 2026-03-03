package main

import (
	"context"
	"testing"
	"time"

	v1 "github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/apis/providerconfig/v1"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	nodeipamconfig "k8s.io/cloud-provider-gcp/pkg/controller/nodeipam/config"
	"k8s.io/cloud-provider/app"
	cloudcontrollerconfig "k8s.io/cloud-provider/app/config"
	genericcontrollermanager "k8s.io/controller-manager/app"
	"k8s.io/controller-manager/pkg/clientbuilder"
)

func TestStartGKETenantControllerManager(t *testing.T) {
	originalEnableProviderConfigController := enableGKETenantController
	defer func() {
		enableGKETenantController = originalEnableProviderConfigController
	}()

	testCases := []struct {
		desc            string
		enable          bool
		wantRun         bool
		wantControllers []string
	}{
		{
			desc:    "disabled",
			enable:  false,
			wantRun: false,
		},
		{
			desc:    "enabled",
			enable:  true,
			wantRun: true,
			wantControllers: []string{
				"node-controller",
				"node-ipam-controller",
				"node-lifecycle-controller",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			enableGKETenantController = tc.enable

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			kubeClient := fake.NewSimpleClientset()
			informerFactory := informers.NewSharedInformerFactory(kubeClient, time.Second)

			initContext := app.ControllerInitContext{
				ClientName: "test-client",
			}

			controllerContext := genericcontrollermanager.ControllerContext{
				InformerFactory: informerFactory,
			}

			ccmConfig := &cloudcontrollerconfig.Config{
				Kubeconfig: &rest.Config{
					Host: "https://example.com",
				},
			}
			ccmConfig.ClientBuilder = clientbuilder.SimpleControllerClientBuilder{
				ClientConfig: ccmConfig.Kubeconfig,
			}
			completedConfig := ccmConfig.Complete()

			cloud := &fakeCloudProvider{}
			nodeIPAMConfig := nodeipamconfig.NodeIPAMControllerConfiguration{}

			_, starter, started, err := startGKETenantControllerManager(gkeTenantControllerManagerConfig{
				ctx:               ctx,
				initContext:       initContext,
				controllerContext: controllerContext,
				completedConfig:   completedConfig,
				cloud:             cloud,
				nodeIPAMConfig:    nodeIPAMConfig,
			})

			if err != nil {
				t.Fatalf("startGKETenantControllerManager failed: %v", err)
			}

			if started != tc.wantRun {
				t.Errorf("startGKETenantControllerManager started = %v, want %v", started, tc.wantRun)
			}

			if tc.wantRun {
				if starter == nil {
					t.Fatal("starter is nil")
				}
				gotControllers := starter.ControllerNames()
				if len(gotControllers) != len(tc.wantControllers) {
					t.Errorf("starter.ControllerNames() = %v, want %v", gotControllers, tc.wantControllers)
				} else {
					for i, name := range gotControllers {
						if name != tc.wantControllers[i] {
							t.Errorf("starter.ControllerNames()[%d] = %s, want %s", i, name, tc.wantControllers[i])
						}
					}
				}
			}
		})
	}
}

func TestGetClusterCIDRsFromProviderConfig(t *testing.T) {
	tests := []struct {
		name          string
		pc            *v1.ProviderConfig
		expectedCIDRs string
	}{
		{
			name: "Single Pod Range",
			pc: &v1.ProviderConfig{
				Spec: v1.ProviderConfigSpec{
					NetworkConfig: v1.ProviderNetworkConfig{
						SubnetInfo: v1.ProviderConfigSubnetInfo{
							PodRanges: []v1.ProviderConfigSecondaryRange{
								{CIDR: "10.100.0.0/16"},
							},
						},
					},
				},
			},
			expectedCIDRs: "10.100.0.0/16",
		},
		{
			name: "Multiple Pod Ranges",
			pc: &v1.ProviderConfig{
				Spec: v1.ProviderConfigSpec{
					NetworkConfig: v1.ProviderNetworkConfig{
						SubnetInfo: v1.ProviderConfigSubnetInfo{
							PodRanges: []v1.ProviderConfigSecondaryRange{
								{CIDR: "10.100.0.0/16"},
								{CIDR: "fd00::/64"},
							},
						},
					},
				},
			},
			expectedCIDRs: "10.100.0.0/16,fd00::/64",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cidrs := getCIDRsFromProviderConfig(tc.pc)
			assert.Equal(t, tc.expectedCIDRs, cidrs)
		})
	}
}
