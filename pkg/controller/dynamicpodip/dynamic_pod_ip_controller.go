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
	"fmt"
	"time"

	nncclientset "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/clientset/versioned"
	nncinformers "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/informers/externalversions/nodenetworkconfig/v1"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	gce "k8s.io/cloud-provider-gcp/providers/gce"
	"k8s.io/utils/clock"
)

const (
	// DefaultBlockSizeMask is the default CIDR mask requested from GCE (e.g. 28 for 16 IPs).
	DefaultBlockSizeMask = 28

	// reconcileTimeout is the maximum time allowed for a single node reconciliation.
	reconcileTimeout = 60 * time.Second
)

var (
	// DefaultBlockSize is the string representation of the default block size (derived from DefaultBlockSizeMask).
	DefaultBlockSize string
	// DefaultCapacity is the number of IPs in the default block size (derived from DefaultBlockSizeMask).
	DefaultCapacity int
)

func init() {
	DefaultCapacity = 1 << (32 - DefaultBlockSizeMask)
	DefaultBlockSize = fmt.Sprintf("/%d", DefaultBlockSizeMask)
}

// Controller is a wrapper around NodeNetworkConfigSpecController and NodeNetworkConfigStatusController.
type Controller struct {
	SpecCtrl   *NodeNetworkConfigSpecController
	StatusCtrl *NodeNetworkConfigStatusController
}

// NewController creates a unified Controller containing both Spec and Status controllers.
func NewController(
	kubeClient kubernetes.Interface,
	nncClient nncclientset.Interface,
	nncInformer nncinformers.NodeNetworkConfigInformer,
	nodeInformer coreinformers.NodeInformer,
	gceCloud *gce.Cloud,
) *Controller {
	loader := func(ctx context.Context, providerID string) ([]*networkInterface, error) {
		gceIfaces, err := gceCloud.GetInstanceNetworkInterfaces(ctx, providerID)
		if err != nil {
			return nil, err
		}
		return toNetworkInterfaces(gceIfaces), nil
	}

	gceCache := NewGCECache(loader, 10*time.Second, clock.RealClock{})

	statusCtrl := NewStatusController(
		kubeClient,
		nncClient,
		nncInformer.Lister(),
		nodeInformer.Lister(),
		gceCloud,
		gceCache,
		clock.RealClock{},
	)

	specCtrl := NewSpecController(
		kubeClient,
		nncClient,
		nncInformer,
		nodeInformer,
		gceCloud,
		gceCache,
		statusCtrl,
	)

	return &Controller{
		SpecCtrl:   specCtrl,
		StatusCtrl: statusCtrl,
	}
}

// Name returns the controller name.
func (c *Controller) Name() string {
	return "dynamic-pod-ip-controller"
}

// Run starts both the Status and Spec controller workers.
func (c *Controller) Run(workers int, stopCh <-chan struct{}) {
	go c.StatusCtrl.Run(workers, stopCh)
	c.SpecCtrl.Run(workers, stopCh)
}
