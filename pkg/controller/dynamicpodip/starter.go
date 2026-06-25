/*
Copyright 2026 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package dynamicpodip

import (
	"context"

	nncclientset "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/clientset/versioned"
	nncinformers "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/informers/externalversions"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/controller-manager/controller"
	gce "k8s.io/cloud-provider-gcp/providers/gce"
	"k8s.io/klog/v2"
)

// StartDynamicPodIPController initializes and starts the Dynamic Pod IP Controller.
func StartDynamicPodIPController(
	ctx context.Context,
	kubeClient kubernetes.Interface,
	nncClient nncclientset.Interface,
	nodeInformer coreinformers.NodeInformer,
	gceCloud *gce.Cloud,
) (controller.Interface, bool, error) {
	klog.Info("Initializing Dynamic Pod IP Controller")

	nncInformerFactory := nncinformers.NewSharedInformerFactory(nncClient, 0)
	nncInformer := nncInformerFactory.Networking().V1().NodeNetworkConfigs()

	ctrl := NewController(
		kubeClient,
		nncClient,
		nncInformer,
		nodeInformer,
		gceCloud,
	)

	// Start the informer factory
	go nncInformerFactory.Start(ctx.Done())

	// Run the controller with 1 worker (sequential processing per node)
	go ctrl.Run(1, ctx.Done())

	return ctrl, true, nil
}
