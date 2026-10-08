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

package ipam

import (
	"context"
	"fmt"

	nncv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/nodenetworkconfig/v1"
	nncclientset "github.com/GoogleCloudPlatform/gke-networking-api/client/nodenetworkconfig/clientset/versioned"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

var (
	// nodeNetworkConfigKeyFun maps node to a namespaced name as key for the task queue.
	nodeNetworkConfigKeyFun = cache.DeletionHandlingMetaNamespaceKeyFunc
)

const (
	// The no. of workers in parallel to update nodenetworkconfig CR
	nodeNetworkConfigWorkers = 30
)

// NodeNetworkConfigSyncer processes nodenetworkconfig CR based on node add/update events.
type NodeNetworkConfigSyncer struct {
	nncClient  nncclientset.Interface
	nodeLister corelisters.NodeLister
}

// NewNodeNetworkConfigSyncer creates a new NodeNetworkConfigSyncer.
func NewNodeNetworkConfigSyncer(nncClient nncclientset.Interface, nodeLister corelisters.NodeLister) *NodeNetworkConfigSyncer {
	return &NodeNetworkConfigSyncer{
		nncClient:  nncClient,
		nodeLister: nodeLister,
	}
}

func (syncer *NodeNetworkConfigSyncer) sync(key string) error {
	klog.V(4).InfoS("Syncing node network config CR for node", "key", key)
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		klog.ErrorS(err, "Failed to split namespace key", "key", key)
		return nil
	}
	return syncer.ensureNodeNetworkConfig(name)
}

// ensureNodeNetworkConfig creates the NodeNetworkConfig CR for nodeName if it does not exist.
func (syncer *NodeNetworkConfigSyncer) ensureNodeNetworkConfig(nodeName string) error {
	ctx := context.Background()
	_, err := syncer.nncClient.NetworkingV1().NodeNetworkConfigs().Get(ctx, nodeName, metav1.GetOptions{})
	if err == nil {
		return nil // Already exists
	}
	if !errors.IsNotFound(err) {
		return fmt.Errorf("failed to get NodeNetworkConfig for node %s: %w", nodeName, err)
	}

	node, err := syncer.nodeLister.Get(nodeName)
	if err != nil {
		if errors.IsNotFound(err) {
			// Node deleted, nothing to do
			return nil
		}
		return fmt.Errorf("failed to get node %s for owner reference: %w", nodeName, err)
	}

	isController := true
	nnc := &nncv1.NodeNetworkConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "v1",
					Kind:       "Node",
					Name:       nodeName,
					UID:        node.UID,
					Controller: &isController,
				},
			},
		},
		Spec: nncv1.NodeNetworkConfigSpec{},
	}
	_, err = syncer.nncClient.NetworkingV1().NodeNetworkConfigs().Create(ctx, nnc, metav1.CreateOptions{})
	if err != nil {
		if errors.IsAlreadyExists(err) {
			klog.V(2).Infof("NodeNetworkConfig was created concurrently for node %s", nodeName)
			return nil
		}
		return fmt.Errorf("failed to create NodeNetworkConfig for node %s: %w", nodeName, err)
	}
	klog.V(2).Infof("Successfully created NodeNetworkConfig CR with owner reference to Node %s", nodeName)
	return nil
}
