/*
Copyright 2026 The Kubernetes Authors.
*/

package gketenantcontrollers

import (
	"fmt"
	"testing"

	v1 "github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/apis/providerconfig/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakedynamic "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"
	cloudcontrollerconfig "k8s.io/cloud-provider/app/config"
	controllermanagerapp "k8s.io/controller-manager/app"
)

func TestNewControllersStarter(t *testing.T) {
	kubeClient := fake.NewSimpleClientset()
	dynamicClient := fakedynamic.NewSimpleDynamicClient(runtime.NewScheme())
	mainInformerFactory := informers.NewSharedInformerFactory(kubeClient, 0)
	config := &cloudcontrollerconfig.CompletedConfig{}
	controlCtx := controllermanagerapp.ControllerContext{}
	controllers := map[string]ControllerStartFunc{}

	starter := NewControllersStarter(nil, kubeClient, dynamicClient, mainInformerFactory, config, controlCtx, controllers)

	if starter == nil {
		t.Fatalf("expected starter to be created, got nil")
	}

	if starter.recorder == nil {
		t.Errorf("expected recorder to be initialized, got nil")
	}
}

func TestRunControllerWithRecovery(t *testing.T) {
	fakeRecorder := record.NewFakeRecorder(10)
	starter := &ControllersStarter{
		recorder: fakeRecorder,
	}

	pc := &v1.ProviderConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-tenant",
		},
	}
	cfg := &ControllerConfig{}

	tests := []struct {
		name           string
		fn             ControllerStartFunc
		expectedEvents []string
	}{
		{
			name: "Normal Execution",
			fn: func(cfg *ControllerConfig) error {
				return nil
			},
			expectedEvents: []string{},
		},
		{
			name: "Controller Returns Error",
			fn: func(cfg *ControllerConfig) error {
				return fmt.Errorf("simulated error")
			},
			expectedEvents: []string{
				"Warning ControllerFailedForTenant Controller test-controller failed: simulated error",
			},
		},
		{
			name: "Controller Panics",
			fn: func(cfg *ControllerConfig) error {
				panic("simulated panic")
			},
			expectedEvents: []string{
				"Warning ControllerFailedForTenant Controller test-controller panicked: simulated panic",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Clear events
			for len(fakeRecorder.Events) > 0 {
				<-fakeRecorder.Events
			}

			// Run controller
			continueLoop, err := starter.runControllerWithRecovery(pc, "test-controller", tc.fn, cfg)

			// verify return values
			if continueLoop != false {
				t.Errorf("expected continueLoop to be false, got %v", continueLoop)
			}
			if err != nil {
				t.Errorf("expected err to be nil, got %v", err)
			}

			// verify events
			for _, expectedEvent := range tc.expectedEvents {
				select {
				case event := <-fakeRecorder.Events:
					if event != expectedEvent {
						t.Errorf("expected event %q, got %q", expectedEvent, event)
					}
				default:
					t.Errorf("expected event %q, but no event was recorded", expectedEvent)
				}
			}

			// verify no extra events
			select {
			case event := <-fakeRecorder.Events:
				t.Errorf("unexpected event: %q", event)
			default:
			}
		})
	}
}
