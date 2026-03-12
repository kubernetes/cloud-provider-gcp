/*
Copyright 2026 The Kubernetes Authors.
*/

package gketenantcontrollers

import (
	"context"
	"sort"
	"time"

	v1 "github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/apis/providerconfig/v1"
	"github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/filtered"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/informers"
	informercorev1 "k8s.io/client-go/informers/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	typev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/cloud-provider-gcp/pkg/controller/gketenantcontrollers/utils"
	cloudcontrollerconfig "k8s.io/cloud-provider/app/config"
	controllermanagerapp "k8s.io/controller-manager/app"
	"k8s.io/klog/v2"
	"k8s.io/utils/clock"
)

const (
	providerConfigLabelKey = "tenancy.gke.io/provider-config"
	conditionReasonFailed  = "ControllerFailedForTenant"
	tenantComponentName    = "gke-tenant-controller-manager"
)

// ControllerConfig contains the dependencies required to start a controller for a specific tenant.
type ControllerConfig struct {
	// Context is the context for the controller. It will be canceled when the tenant is removed.
	Context context.Context
	// Cloud is the tenant-scoped cloud provider.
	Cloud cloudprovider.Interface
	// NodeInformer is the filtered node informer for this tenant.
	NodeInformer informercorev1.NodeInformer
	// KubeClient is the kubernetes client.
	KubeClient clientset.Interface
	// ProviderConfig is the ProviderConfig object triggering this controller.
	ProviderConfig *v1.ProviderConfig
	// ControllerContext is the controller manager context (contains metrics, etc.)
	ControllerContext controllermanagerapp.ControllerContext
}

// ControllerStartFunc is a function that starts a controller.
// It should block until the context is canceled or an error occurs.
type ControllerStartFunc func(cfg *ControllerConfig) error

// ControllersStarter implements the framework.ControllerStarter interface.
type ControllersStarter struct {
	clientBuilder       cloudprovider.ControllerClientBuilder
	kubeClient          clientset.Interface
	dynamicClient       dynamic.Interface
	mainInformerFactory informers.SharedInformerFactory
	config              *cloudcontrollerconfig.CompletedConfig
	controllerCtx       controllermanagerapp.ControllerContext
	controllers         map[string]ControllerStartFunc
	recorder            record.EventRecorder
}

// NewControllersStarter creates a new ControllersStarter.
func NewControllersStarter(
	clientBuilder cloudprovider.ControllerClientBuilder,
	kubeClient clientset.Interface,
	dynamicClient dynamic.Interface,
	mainInformerFactory informers.SharedInformerFactory,
	config *cloudcontrollerconfig.CompletedConfig,
	controllerCtx controllermanagerapp.ControllerContext,
	controllers map[string]ControllerStartFunc,
) *ControllersStarter {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&typev1.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})
	scheme := runtime.NewScheme()
	v1.AddToScheme(scheme)
	recorder := eventBroadcaster.NewRecorder(scheme, corev1.EventSource{Component: tenantComponentName})

	return &ControllersStarter{
		clientBuilder:       clientBuilder,
		kubeClient:          kubeClient,
		dynamicClient:       dynamicClient,
		mainInformerFactory: mainInformerFactory,
		config:              config,
		controllerCtx:       controllerCtx,
		controllers:         controllers,
		recorder:            recorder,
	}
}

// ControllerNames returns the names of the controllers that will be started.
func (s *ControllersStarter) ControllerNames() []string {
	var names []string
	for name := range s.controllers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// StartController starts a map of new scoped controllers for the given ProviderConfig.
// It returns a release channel that can be closed to stop the controller.
func (s *ControllersStarter) StartController(pc *v1.ProviderConfig) (chan<- struct{}, error) {
	pcKey := pc.Name
	stopCh := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-stopCh
		cancel()
	}()

	klog.Infof("[%s] Attempting to start scoped controller", pcKey)
	var filteredFactory *filtered.FilteredSharedInformerFactory

	// Initialize asynchronously to avoid blocking the framework's event loop.
	go func() {
		// defer to cleanup the resources when the controller is stopped.
		defer func() {
			klog.Infof("[%s] Scoped controller stopping", pcKey)
			cancel()
			if filteredFactory != nil {
				filteredFactory.Cleanup()
			}
		}()

		// Create the new, scoped GCECloud object
		klog.V(2).Infof("[%s] Creating tenant-scoped GCE cloud object...", pcKey)
		scopedCloud, err := CreateTenantScopedGCECloud(s.config, pc)
		if err != nil {
			klog.Errorf("[%s] Failed to create scoped cloud: %v. Aborting controller startup.", pcKey, err)
			return
		}
		klog.Infof("[%s] Created scoped cloud successfully: %+v", pcKey, scopedCloud)

		// Create filtered informer factory for the tenant
		klog.V(2).Infof("[%s] Creating filtered informer factory...", pcKey)
		// allow nodes with missing label if this is the supervisor controller
		allowMissing := utils.IsSupervisor(pc)
		filteredFactory = filtered.NewFilteredSharedInformerFactory(s.mainInformerFactory, providerConfigLabelKey, pcKey, allowMissing)

		if informerUserCloud, ok := scopedCloud.(cloudprovider.InformerUser); ok {
			klog.Infof("[%s] Setting up informers for scoped cloud", pcKey)
			informerUserCloud.SetInformers(filteredFactory)
		}

		nodeInformer := filteredFactory.Core().V1().Nodes()

		klog.Infof("[%s] Waiting for main informer caches to sync...", pcKey)
		if !cache.WaitForCacheSync(ctx.Done(),
			s.mainInformerFactory.Core().V1().Nodes().Informer().HasSynced,
		) {
			klog.Errorf("[%s] Failed to sync main caches. Aborting controller startup.", pcKey)
			return
		}
		klog.Infof("[%s] Main informer caches synced successfully", pcKey)

		// Create ControllerConfig
		cfg := &ControllerConfig{
			Context:           ctx,
			Cloud:             scopedCloud,
			NodeInformer:      nodeInformer,
			KubeClient:        s.kubeClient,
			ProviderConfig:    pc,
			ControllerContext: s.controllerCtx,
		}

		// Start all registered controllers
		for name, startFunc := range s.controllers {
			klog.Infof("[%s] Starting controller: %s", pcKey, name)
			// Launch each controller in a separate goroutine
			go func(controllerName string, fn ControllerStartFunc) {
				// Create an exponential backoff manager
				// Starts at 1s, doubles each time, up to a max of 5 minutes.
				backoff := wait.Backoff{
					Duration: 1 * time.Second,
					Factor:   2.0,
					Jitter:   0.0,
					Steps:    1000000, // Effectively infinite steps as we don't want to stop retrying
					Cap:      5 * time.Minute,
				}
				delayFn := backoff.DelayWithReset(clock.RealClock{}, 2*time.Minute)
				// Use DelayFunc.Until which handles the timer internally
				delayFn.Until(ctx, true, true, func(ctx context.Context) (bool, error) {
					return s.runControllerWithRecovery(pc, controllerName, fn, cfg)
				})
			}(name, startFunc)
		}
		// Block until context is canceled
		<-ctx.Done()
	}()

	return stopCh, nil
}

// runControllerWithRecovery executes the controller function and recovers from any panics.
// It also logs errors and emits events on failure or panic.
// It returns (false, nil) to emulate BackoffUntil's infinite loop behavior until context is cancelled.
func (s *ControllersStarter) runControllerWithRecovery(pc *v1.ProviderConfig, controllerName string, fn ControllerStartFunc, cfg *ControllerConfig) (bool, error) {
	pcKey := pc.Name
	defer func() {
		if r := recover(); r != nil {
			klog.Errorf("[%s] Controller %s panicked: %v", pcKey, controllerName, r)
			s.recorder.Eventf(pc, corev1.EventTypeWarning, conditionReasonFailed, "Controller %s panicked: %v", controllerName, r)
		}
	}()

	klog.Infof("[%s] Starting controller %s", pcKey, controllerName)
	if err := fn(cfg); err != nil {
		klog.Errorf("[%s] Controller %s failed: %v", pcKey, controllerName, err)
		s.recorder.Eventf(pc, corev1.EventTypeWarning, conditionReasonFailed, "Controller %s failed: %v", controllerName, err)
	} else {
		klog.Infof("[%s] Controller %s exited normally", pcKey, controllerName)
	}
	// Return false, nil to emulate BackoffUntil's infinite loop behavior (until context cancelled)
	return false, nil
}
