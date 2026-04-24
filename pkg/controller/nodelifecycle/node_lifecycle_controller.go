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

package nodelifecycle

import (
	"time"

	coreinformers "k8s.io/client-go/informers/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/cloud-provider/controllers/nodelifecycle"
	"k8s.io/cloud-provider-gcp/pkg/util/node"
)

// NewCloudNodeLifecycleController returns a new cloud node lifecycle controller that filters out unmanaged nodes.
func NewCloudNodeLifecycleController(
	nodeInformer coreinformers.NodeInformer,
	kubeClient clientset.Interface,
	cloud cloudprovider.Interface,
	nodeMonitorPeriod time.Duration) (*nodelifecycle.CloudNodeLifecycleController, error) {

	// Wrap the informer to filter nodes
	filteringInformer := &node.GCEFilteringNodeInformer{NodeInformer: nodeInformer}

	return nodelifecycle.NewCloudNodeLifecycleController(
		filteringInformer,
		kubeClient,
		cloud,
		nodeMonitorPeriod,
	)
}
