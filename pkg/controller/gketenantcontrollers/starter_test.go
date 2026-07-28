/*
Copyright 2026 The Kubernetes Authors.
*/

package gketenantcontrollers

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	v1 "github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/apis/providerconfig/v1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakedynamic "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"
	cloudprovider "k8s.io/cloud-provider"
	gce "k8s.io/cloud-provider-gcp/providers/gce"
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

func TestStartController_CloudClientRetry(t *testing.T) {
	pc := &v1.ProviderConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "retry-tenant",
		},
	}

	tests := []struct {
		name          string
		mockCreateFn  func(attempts *int32) func(*cloudcontrollerconfig.CompletedConfig, *v1.ProviderConfig) (cloudprovider.Interface, error)
		expectSuccess bool
		minAttempts   int
		timeout       time.Duration
	}{
		{
			name: "Succeeds after 2 failures",
			mockCreateFn: func(attempts *int32) func(*cloudcontrollerconfig.CompletedConfig, *v1.ProviderConfig) (cloudprovider.Interface, error) {
				return func(config *cloudcontrollerconfig.CompletedConfig, pc *v1.ProviderConfig) (cloudprovider.Interface, error) {
					curr := atomic.AddInt32(attempts, 1)
					if curr <= 2 {
						return nil, fmt.Errorf("transient GCE error")
					}
					return &gce.Cloud{}, nil // Mock Cloud
				}
			},
			expectSuccess: true,
			minAttempts:   3,
			timeout:       15 * time.Second,
		},
		{
			name: "Fails permanently and times out",
			mockCreateFn: func(attempts *int32) func(*cloudcontrollerconfig.CompletedConfig, *v1.ProviderConfig) (cloudprovider.Interface, error) {
				return func(config *cloudcontrollerconfig.CompletedConfig, pc *v1.ProviderConfig) (cloudprovider.Interface, error) {
					atomic.AddInt32(attempts, 1)
					return nil, fmt.Errorf("permanent GCE error")
				}
			},
			expectSuccess: false,
			minAttempts:   3,               // inside a short timeout context, it will still retry a few times with 1s start
			timeout:       8 * time.Second, // test short timeout
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			kubeClient := fake.NewSimpleClientset()
			dynamicClient := fakedynamic.NewSimpleDynamicClient(runtime.NewScheme())
			mainInformerFactory := informers.NewSharedInformerFactory(kubeClient, 0)
			eventBroadcaster := record.NewBroadcaster()

			scheme := runtime.NewScheme()
			_ = v1.AddToScheme(scheme)
			fakeRecorder := eventBroadcaster.NewRecorder(scheme, corev1.EventSource{Component: "test"})

			starter := &ControllersStarter{
				kubeClient:          kubeClient,
				dynamicClient:       dynamicClient,
				mainInformerFactory: mainInformerFactory,
				config:              &cloudcontrollerconfig.CompletedConfig{},
				recorder:            fakeRecorder,
				controllers:         map[string]ControllerStartFunc{},
			}

			var attempts int32
			starter.createCloudFn = tc.mockCreateFn(&attempts)

			starter.clientCreationTimeout = tc.timeout

			// Start the informer factory so WaitForCacheSync can succeed in the success case
			stopCh := make(chan struct{})
			defer close(stopCh)
			mainInformerFactory.Start(stopCh)

			// Start the controller asynchronously
			runStopCh, err := starter.StartController(pc)
			assert.NoError(t, err)
			defer close(runStopCh)

			if tc.expectSuccess {
				// Poll for attempts to reach expected count
				assert.Eventually(t, func() bool {
					return atomic.LoadInt32(&attempts) >= int32(tc.minAttempts)
				}, 10*time.Second, 100*time.Millisecond, "expected at least %d attempts, got %d", tc.minAttempts, atomic.LoadInt32(&attempts))

				// Wait to ensure no further retries occur after success
				time.Sleep(2 * time.Second)
				finalAttempts := atomic.LoadInt32(&attempts)
				assert.Equal(t, int32(tc.minAttempts), finalAttempts, "should not retry after success")
			} else {
				// Wait for the short timeout to expire
				time.Sleep(tc.timeout + 2*time.Second)
				finalAttempts := atomic.LoadInt32(&attempts)
				assert.GreaterOrEqual(t, finalAttempts, int32(tc.minAttempts), "expected at least %d attempts before timing out, got %d", tc.minAttempts, finalAttempts)
			}
		})
	}
}
