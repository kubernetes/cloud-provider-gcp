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

package dynamicpodip

import (
	"context"
	"sync"
	"time"

	nncclientset "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/clientset/versioned"
	nncinformers "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/informers/externalversions"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/controller-manager/controller"
	gce "k8s.io/cloud-provider-gcp/providers/gce"
	"k8s.io/klog/v2"
	"k8s.io/utils/clock"
)

var (
	globalStatusTriggerMu sync.RWMutex
	globalStatusTrigger   StatusTrigger = &NoopStatusTrigger{}
)

// GetStatusTrigger returns the active StatusTrigger (or NoopStatusTrigger if not initialized).
func GetStatusTrigger() StatusTrigger {
	globalStatusTriggerMu.RLock()
	defer globalStatusTriggerMu.RUnlock()
	return globalStatusTrigger
}

// SetStatusTrigger sets the active StatusTrigger.
func SetStatusTrigger(st StatusTrigger) {
	globalStatusTriggerMu.Lock()
	defer globalStatusTriggerMu.Unlock()
	if st == nil {
		globalStatusTrigger = &NoopStatusTrigger{}
	} else {
		globalStatusTrigger = st
	}
}

// Options holds activation flags for the NodeNetworkConfig controllers.
type Options struct {
	// PopulateNodeNetworkConfig activates the "read" side (NodeNetworkConfigStatusController).
	PopulateNodeNetworkConfig bool
	// EnableDynamicPodIPController activates the "write" side (NodeNetworkConfigSpecController).
	// Enabling this implicitly forces PopulateNodeNetworkConfig to true.
	EnableDynamicPodIPController bool
}

// StartControllers initializes and starts the status and/or spec controllers based on the provided options.
// Returns the status trigger interface (which can be passed to Node IPAM or other callers), whether any controller was started, and any error.
func StartControllers(
	ctx context.Context,
	opts Options,
	kubeClient kubernetes.Interface,
	nncClient nncclientset.Interface,
	nodeInformer coreinformers.NodeInformer,
	gceCloud *gce.Cloud,
) (StatusTrigger, controller.Interface, bool, error) {
	// --enable-dynamic-pod-ip-controller implies --populate-node-network-config=true
	if opts.EnableDynamicPodIPController {
		opts.PopulateNodeNetworkConfig = true
	}

	if !opts.PopulateNodeNetworkConfig && !opts.EnableDynamicPodIPController {
		klog.Info("Neither --populate-node-network-config nor --enable-dynamic-pod-ip-controller is set; dynamic pod IP controllers will not be started")
		return GetStatusTrigger(), nil, false, nil
	}

	nncInformerFactory := nncinformers.NewSharedInformerFactory(nncClient, 0)
	nncInformer := nncInformerFactory.Networking().V1().NodeNetworkConfigs()

	loader := func(ctx context.Context, providerID string) ([]*networkInterface, error) {
		gceIfaces, err := gceCloud.GetInstanceNetworkInterfaces(ctx, providerID)
		if err != nil {
			return nil, err
		}
		return toNetworkInterfaces(gceIfaces), nil
	}

	gceCache := NewGCECache(loader, 10*time.Second, clock.RealClock{})

	var statusTrigger StatusTrigger = &NoopStatusTrigger{}
	var statusCtrl *NodeNetworkConfigStatusController

	if opts.PopulateNodeNetworkConfig {
		klog.Info("Initializing NodeNetworkConfig Status Controller")
		statusCtrl = NewStatusController(
			kubeClient,
			nncClient,
			nncInformer.Lister(),
			nodeInformer.Lister(),
			gceCloud,
			gceCache,
			clock.RealClock{},
		)
		statusTrigger = statusCtrl
		SetStatusTrigger(statusTrigger)
	}

	if opts.EnableDynamicPodIPController {
		klog.Info("Initializing NodeNetworkConfig Spec Controller")
		specCtrl := NewSpecController(
			kubeClient,
			nncClient,
			nncInformer,
			nodeInformer,
			gceCloud,
			gceCache,
			statusTrigger,
		)
		go specCtrl.Run(DefaultSpecControllerWorkers, ctx.Done())
	}

	if statusCtrl != nil {
		go statusCtrl.Run(DefaultStatusControllerWorkers, ctx.Done())
	}

	go nncInformerFactory.Start(ctx.Done())

	return statusTrigger, statusCtrl, true, nil
}

// StartDynamicPodIPController is a backwards-compatible helper that starts both controllers with --enable-dynamic-pod-ip-controller=true.
func StartDynamicPodIPController(
	ctx context.Context,
	kubeClient kubernetes.Interface,
	nncClient nncclientset.Interface,
	nodeInformer coreinformers.NodeInformer,
	gceCloud *gce.Cloud,
) (controller.Interface, bool, error) {
	_, ctrl, started, err := StartControllers(
		ctx,
		Options{
			EnableDynamicPodIPController: true,
		},
		kubeClient,
		nncClient,
		nodeInformer,
		gceCloud,
	)
	return ctrl, started, err
}
