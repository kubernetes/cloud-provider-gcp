/*
Copyright 2023 The Kubernetes Authors.

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

// Package nodesyncer listens to and process node sync events. For each node sync
// event, it gets all pods on that node, get all verified GSAs of the pods, and
// sync it (setSecondaryServiceAccounts).
package nodesyncer

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"sync"
	"time"

	core "k8s.io/api/core/v1"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/tools/cache"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/cloud-provider-gcp/cmd/gcp-controller-manager/dpwi/auth"
	"k8s.io/cloud-provider-gcp/cmd/gcp-controller-manager/dpwi/ctxlog"
	"k8s.io/cloud-provider-gcp/cmd/gcp-controller-manager/dpwi/eventhandler"
	"k8s.io/cloud-provider-gcp/cmd/gcp-controller-manager/dpwi/serviceaccounts"
)

const (
	eventHandlerResyncPeriod = 30 * time.Minute
	indexByNode              = "index-by-node"
)

// NodeHandler handles node events.
type NodeHandler struct {
	eventhandler.EventHandler
	podIndexer  cache.Indexer
	nodeIndexer cache.Indexer
	queue       workqueue.RateLimitingInterface
	verifier    verifier
	auth        *auth.Client
	nodeMap     *nodeMap
}

type verifier interface {
	VerifiedGSA(ctx context.Context, ksa serviceaccounts.ServiceAccount) (serviceaccounts.GSAEmail, error)
}

// NewEventHandler creates a new eventHandler.
func NewEventHandler(podInformer coreinformers.PodInformer, nodeInformer coreinformers.NodeInformer, verifier verifier, authSyncNodeURL, hmsSyncNodeURL string) (*NodeHandler, error) {
	auth, err := auth.NewClient(authSyncNodeURL, hmsSyncNodeURL, &clientcmdapi.AuthProviderConfig{Name: "gcp"})
	if err != nil {
		return nil, err
	}
	podIndexer := podInformer.Informer().GetIndexer()
	podIndexer.AddIndexers(map[string]cache.IndexFunc{
		indexByNode: func(obj interface{}) ([]string, error) {
			pod, ok := obj.(*core.Pod)
			if !ok {
				return nil, fmt.Errorf("invalid pod object from key %v", obj)
			}
			node := pod.Spec.NodeName
			return []string{node}, nil
		},
	})
	nh := &NodeHandler{
		podIndexer:  podIndexer,
		verifier:    verifier,
		auth:        auth,
		nodeIndexer: nodeInformer.Informer().GetIndexer(),
		nodeMap:     &nodeMap{m: make(map[string]*nodeGSAs)},
	}
	nh.InitEventHandler("node-syncer", nh.process)
	nodeInformer.Informer().AddEventHandlerWithResyncPeriod(nh.ResourceEventHandler(), eventHandlerResyncPeriod)
	return nh, nil
}

func (nh *NodeHandler) process(ctx context.Context, key string) error {
	oNode, exist, err := nh.nodeIndexer.GetByKey(key)
	if err != nil {
		return err
	}
	if !exist {
		ctxlog.Infof(ctx, "Node %q doesn't exist. Delete it from nodeMap", key)
		nh.nodeMap.delete(key)
		return nil
	}

	node, ok := oNode.(*core.Node)
	if !ok {
		return fmt.Errorf("invalid node object from key %q: %#v", key, oNode)
	}
	zone, ok := node.ObjectMeta.Labels[core.LabelTopologyZone]
	if !ok || zone == "" {
		return fmt.Errorf("failed to get zone from node %q", key)
	}

	pods, err := nh.podIndexer.ByIndex(indexByNode, key)
	if err != nil {
		return fmt.Errorf("failed to get pods on the node %q: %w", key, err)
	}

	gsaSet := make(map[string]int)
	for _, p := range pods {
		gsa, err := nh.gsaForPod(ctx, p)
		// Don't let some failures block the whole node from syncing.
		// A pod failure here means that the pod event is still in processing or
		// in the backlog of the Pod event handler. Once the pod is processed successfully,
		// it will send another node sync event.
		if err != nil {
			ctxlog.Warningf(ctx, "Ignore the failure of getting gsa: %v", err)
			continue
		}
		if gsa == "" {
			continue
		}
		gsaSet[string(gsa)]++
	}
	if len(gsaSet) == 0 {
		return nil
	}
	var curGSAs []string
	for k := range gsaSet {
		curGSAs = append(curGSAs, k)
	}
	return nh.nodeMap.getNodeGSAs(key).sync(
		ctx,
		nh.auth,
		zone,
		curGSAs,
	)
}

func (nh *NodeHandler) gsaForPod(ctx context.Context, p interface{}) (serviceaccounts.GSAEmail, error) {
	pod, ok := p.(*core.Pod)
	if !ok {
		return "", fmt.Errorf("invalid pod object %v", p)
	}

	ksa := serviceaccounts.ServiceAccount{
		Namespace: pod.ObjectMeta.Namespace,
		Name:      pod.Spec.ServiceAccountName,
	}
	gsa, err := nh.verifier.VerifiedGSA(ctx, ksa)
	if err != nil {
		err = fmt.Errorf("error getting verified GSA for ksa %v for pod %q: %w", ksa, pod.Name, err)
	}
	return gsa, err
}

type nodeMap struct {
	sync.Mutex
	m map[string]*nodeGSAs
}

func (nm *nodeMap) delete(node string) {
	nm.Lock()
	defer nm.Unlock()
	delete(nm.m, node)
}

func (nm *nodeMap) getNodeGSAs(node string) *nodeGSAs {
	nm.Lock()
	defer nm.Unlock()
	if v, ok := nm.m[node]; ok {
		return v
	}
	entry := &nodeGSAs{
		node: node,
	}
	nm.m[node] = entry
	return entry
}

type nodeGSAs struct {
	sync.Mutex
	lastGSAs []string
	lastSent time.Time
	node     string
}

func (ng *nodeGSAs) sync(ctx context.Context, client *auth.Client, zone string, curGSAs []string) error {
	ng.Lock()
	defer ng.Unlock()
	sort.Strings(curGSAs)
	if reflect.DeepEqual(ng.lastGSAs, curGSAs) && time.Since(ng.lastSent) < eventHandlerResyncPeriod {
		ctxlog.Infof(ctx, "Don't sync as the GSA list is the same.")
		return nil
	}

	// Don't send too many requests to down-streams.
	if since := time.Since(ng.lastSent); since < time.Second*10 {
		return fmt.Errorf("the node %q was synced less than %v ago, which is less than 10s", ng.node, since)
	}

	err := client.Sync(ctx, ng.node, zone, curGSAs)
	if err != nil {
		return err
	}
	ctxlog.Infof(ctx, "Sent GSAs: %v", curGSAs)
	ng.lastGSAs = curGSAs
	ng.lastSent = time.Now()
	return nil
}
