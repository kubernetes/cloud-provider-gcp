package main

import (
	"context"
	"testing"
	"time"

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

			_, starter, started, err := startGKETenantControllerManager(
				ctx,
				initContext,
				controllerContext,
				completedConfig,
				cloud,
				nodeIPAMConfig,
			)

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
