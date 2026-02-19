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
	originalEnableProviderConfigController := enableProviderConfigController
	defer func() {
		enableProviderConfigController = originalEnableProviderConfigController
	}()

	testCases := []struct {
		desc    string
		enable  bool
		wantRun bool
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
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			enableProviderConfigController = tc.enable

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

			_, started, err := startGKETenantControllerManager(
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
		})
	}
}
