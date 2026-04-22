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

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	coreinformers "k8s.io/client-go/informers/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	v1lister "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/cloud-provider/controllers/nodelifecycle"
)

const (
	// GKEUnmanagedNodeLabelKey is the label key used to identify nodes that should be ignored by the node lifecycle manager.
	GKEUnmanagedNodeLabelKey = "cloud.google.com/gke-unmanaged-node"
	// GKEUnmanagedNodeLabelValue is the label value used to identify nodes that should be ignored by the node lifecycle manager.
	GKEUnmanagedNodeLabelValue = "true"
)

// GCEFilteringNodeInformer wraps a NodeInformer to filter out unmanaged nodes from the lister.
type GCEFilteringNodeInformer struct {
	coreinformers.NodeInformer
}

func (i *GCEFilteringNodeInformer) Lister() v1lister.NodeLister {
	return &GCEFilteringNodeLister{i.NodeInformer.Lister()}
}

func (i *GCEFilteringNodeInformer) Informer() cache.SharedIndexInformer {
	return i.NodeInformer.Informer()
}

// GCEFilteringNodeLister wraps a NodeLister to filter out nodes with the unmanaged label.
type GCEFilteringNodeLister struct {
	v1lister.NodeLister
}

func (l *GCEFilteringNodeLister) List(selector labels.Selector) (ret []*v1.Node, err error) {
	nodes, err := l.NodeLister.List(selector)
	if err != nil {
		return nil, err
	}
	var filtered []*v1.Node
	for _, n := range nodes {
		if val, ok := n.Labels[GKEUnmanagedNodeLabelKey]; ok && val == GKEUnmanagedNodeLabelValue {
			continue
		}
		filtered = append(filtered, n)
	}
	return filtered, nil
}

func (l *GCEFilteringNodeLister) Get(name string) (*v1.Node, error) {
	return l.NodeLister.Get(name)
}

// NewCloudNodeLifecycleController returns a new cloud node lifecycle controller that filters out unmanaged nodes.
func NewCloudNodeLifecycleController(
	nodeInformer coreinformers.NodeInformer,
	kubeClient clientset.Interface,
	cloud cloudprovider.Interface,
	nodeMonitorPeriod time.Duration) (*nodelifecycle.CloudNodeLifecycleController, error) {

	// Wrap the informer to filter nodes
	filteringInformer := &GCEFilteringNodeInformer{NodeInformer: nodeInformer}

	return nodelifecycle.NewCloudNodeLifecycleController(
		filteringInformer,
		kubeClient,
		cloud,
		nodeMonitorPeriod,
	)
}
