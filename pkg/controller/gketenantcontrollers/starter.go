/*
Copyright 2026 The Kubernetes Authors.
*/

package gketenantcontrollers

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"k8s.io/client-go/informers"
	corev1 "k8s.io/client-go/informers/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	cloudprovider "k8s.io/cloud-provider"
	cloudcontrollerconfig "k8s.io/cloud-provider/app/config"
	controllermanagerapp "k8s.io/controller-manager/app"

	// This is the OSS Node Controller package
	v1 "github.com/GoogleCloudPlatform/gke-enterprise-mt/apis/providerconfig/v1"
	"k8s.io/klog/v2"
)

// ControllerConfig contains the dependencies required to start a controller for a specific tenant.
type ControllerConfig struct {
	// Context is the context for the controller. It will be canceled when the tenant is removed.
	Context context.Context
	// Cloud is the tenant-scoped cloud provider.
	Cloud cloudprovider.Interface
	// NodeInformer is the filtered node informer for this tenant.
	NodeInformer corev1.NodeInformer
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
	mainInformerFactory informers.SharedInformerFactory
	config              *cloudcontrollerconfig.CompletedConfig
	controlCtx          controllermanagerapp.ControllerContext
	controllers         map[string]ControllerStartFunc
}

// NewControllersStarter creates a new ControllersStarter.
func NewControllersStarter(
	clientBuilder cloudprovider.ControllerClientBuilder,
	kubeClient clientset.Interface,
	mainInformerFactory informers.SharedInformerFactory,
	config *cloudcontrollerconfig.CompletedConfig,
	controlCtx controllermanagerapp.ControllerContext,
	controllers map[string]ControllerStartFunc,
) *ControllersStarter {
	return &ControllersStarter{
		clientBuilder:       clientBuilder,
		kubeClient:          kubeClient,
		mainInformerFactory: mainInformerFactory,
		config:              config,
		controlCtx:          controlCtx,
		controllers:         controllers,
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

// StartController starts a new scoped node controller for the given ProviderConfig.
// It returns a release channel that can be closed to stop the controller.
func (s *ControllersStarter) StartController(pc *v1.ProviderConfig) (chan<- struct{}, error) {
	pcKey := pc.Name
	stopCh := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-stopCh
		cancel()
	}()

	klog.Infof("[%s] Attempting to start scoped node controller", pcKey)
	var filteredFactory *filteredSharedInformerFactory

	// Note: We run this in a goroutine because StartController must be non-blocking.
	// BUT the framework expects us to return a channel to stop it.
	go func() {
		defer func() {
			klog.Infof("[%s] Scoped node controller stopping", pcKey)
			cancel()
			// Cleanup event handlers
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

		providerConfigName, err := getNodeLabelSelector(pc)
		if err != nil {
			klog.Errorf("[%s] Failed to get node label selector: %v. Aborting controller startup.", pcKey, err)
			return
		}
		klog.Infof("[%s] Using node label selector: %s", pcKey, providerConfigName)

		klog.V(2).Infof("[%s] Creating filtered informer factory...", pcKey)
		// allow nodes with missing label if this is the supervisor controller
		allowMissing := strings.HasPrefix(pcKey, "s")
		filteredFactory = newFilteredSharedInformerFactory(s.mainInformerFactory, providerConfigLabelKey, providerConfigName, allowMissing)

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
			ControllerContext: s.controlCtx,
		}

		// Start all registered controllers
		for name, startFunc := range s.controllers {
			klog.Infof("[%s] Starting controller: %s", pcKey, name)
			// Launch each controller in a separate goroutine
			go func(controllerName string, fn ControllerStartFunc) {
				defer func() {
					if r := recover(); r != nil {
						klog.Errorf("[%s] Controller %s panicked: %v", pcKey, controllerName, r)
					}
				}()
				if err := fn(cfg); err != nil {
					klog.Errorf("[%s] Controller %s failed: %v", pcKey, controllerName, err)
				}
				klog.Infof("[%s] Controller %s exited", pcKey, controllerName)
			}(name, startFunc)
		}
		// Block until context is canceled
		<-ctx.Done()
	}()

	return stopCh, nil
}

// GetClusterCIDRsFromProviderConfig returns a comma-separated list of CIDRs from the given ProviderConfig.
func GetClusterCIDRsFromProviderConfig(pc *v1.ProviderConfig) (string, error) {
	if len(pc.Spec.NetworkConfig.SubnetInfo.PodRanges) == 0 {
		return "", fmt.Errorf("no pod ranges found in provider config")
	}

	var cidrs []string
	for _, podRange := range pc.Spec.NetworkConfig.SubnetInfo.PodRanges {
		if podRange.CIDR == "" {
			continue
		}
		cidrs = append(cidrs, podRange.CIDR)
	}

	if len(cidrs) == 0 {
		return "", fmt.Errorf("all pod ranges in provider config have empty CIDR")
	}

	return strings.Join(cidrs, ","), nil
}
