/*
Copyright 2019 The Kubernetes Authors.

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

package main

import (
	"fmt"
	"sync"
	"time"

	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/tools/cache"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

const (
	nodeSyncerControlLoopName = "node-syncer"
	nodeSyncerQueueName       = "node-syncer-queue"
	nodeSyncerResyncPeriod    = 30 * time.Minute
)

// nodeSyncer implements a custom control loop responsible for synchronizing GCE with the list of
// authorized and in-use GCP Service Accounts (GSA) on each Node.  GSA authorization is handled by
// the service-account-verifier control loop which shares the result over the verifiedSAs map.  Use
// of individual authorized GSA on each node is tracked by this control loop based on Pod events.
type nodeSyncer struct {
	indexer     cache.Indexer
	hasSynced   func() bool
	queue       workqueue.RateLimitingInterface
	nodes       *nodeMap
	verifiedSAs *saMap
	hms         *hmsClient
	location    string
}

func newNodeSyncer(informer coreinformers.PodInformer, sm *saMap, hmsSyncNodeURL, location string) (*nodeSyncer, error) {
	hms, err := newHMSClient(hmsSyncNodeURL, &clientcmdapi.AuthProviderConfig{Name: "gcp"})
	if err != nil {
		return nil, err
	}
	ns := &nodeSyncer{
		indexer:   informer.Informer().GetIndexer(),
		hasSynced: informer.Informer().HasSynced,
		queue: workqueue.NewNamedRateLimitingQueue(workqueue.NewMaxOfRateLimiter(
			workqueue.NewItemExponentialFailureRateLimiter(200*time.Millisecond, 1000*time.Second),
		), nodeSyncerQueueName),
		nodes:       newNodeMap(),
		verifiedSAs: sm,
		hms:         hms,
		location:    location,
	}
	informer.Informer().AddEventHandlerWithResyncPeriod(cache.ResourceEventHandlerFuncs{
		AddFunc:    ns.onPodAdd,
		UpdateFunc: ns.onPodUpdate,
		DeleteFunc: ns.onPodDelete,
	}, nodeSyncerResyncPeriod)
	return ns, nil
}

func (ns *nodeSyncer) onPodAdd(obj interface{}) {
	ns.enqueue(obj)
}

func (ns *nodeSyncer) onPodUpdate(obj, oldObj interface{}) {
	ns.enqueue(obj)
}

func (ns *nodeSyncer) onPodDelete(obj interface{}) {
	ns.enqueue(obj)
}

func (ns *nodeSyncer) enqueue(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		klog.Errorf("internal error. Couldn't get key for Pod %+v: %v", obj, err)
		return
	}
	ns.queue.AddRateLimited(key)
}

func (ns *nodeSyncer) Run(workers int, stopCh <-chan struct{}) {
	// TODO(danielywong): block on ServiceAccount HasSync to give time for verifiedSAs being populated.
	if !cache.WaitForNamedCacheSync(nodeSyncerControlLoopName, stopCh, ns.hasSynced) {
		return
	}
	for i := 0; i < workers; i++ {
		go wait.Until(ns.work, time.Second, stopCh)
	}
	<-stopCh
}

func (ns *nodeSyncer) work() {
	for ns.processNext() {
	}
}

func (ns *nodeSyncer) processNext() bool {
	key, quit := ns.queue.Get()
	if quit {
		return false
	}
	defer ns.queue.Done(key)

	err := ns.process(key.(string))
	if err != nil {
		klog.Warningf("Requeuing %q due to %v", key, err)
		ns.queue.AddRateLimited(key)
		return true
	}
	ns.queue.Forget(key)
	return true
}

// Process processes Pod events to maintain the list of GSAs that are authorized by
// service-account-verifier and in-use by running Pods for each node in the cluster.
//
// First, it reads the pod's KSA and looks up the authorized GSA for that ServiceAccount from the
// verifiedSAs map that service-account-verifier maintains.  The (pod, GSA) pair is then
// added to the nodeMap table which is keyed by the name of the node where the pod is running.
//
// In each nodeMap entry, the validated GSA and the pod's workqueue key (ie, <namespace name>/<pod
// name>) are stored in a map indexed by the pod object's UID.
//
// TODO(danielywong): Call ns.sync only if necessary; that is, first use of a GSA on Pod addition or
// last use on Pod delete.
func (ns *nodeSyncer) process(key string) error {
	o, exists, err := ns.indexer.GetByKey(key)
	if err != nil {
		return fmt.Errorf("failed to get Pod %q: %v", key, err)
	}
	if !exists {
		syncNodes := ns.nodes.remove(key)
		klog.Infof("Pod %q being removed was found in %v nodes", key, len(syncNodes))
		for node, gsa := range syncNodes {
			klog.Infof("Pod %q and GSA %q are removed from Node %q", key, gsa, node)
			if err := ns.sync(node); err != nil {
				// Log only; retries will be triggered by informer's resync events.
				klog.Warningf("Failed to sync Node %q: %v", node, err)
			}
		}
		return nil
	}
	pod, ok := o.(*core.Pod)
	if !ok {
		return fmt.Errorf("invalid pod object from key %q: %#v", key, o)
	}

	ksa := serviceAccount{
		Namespace: pod.ObjectMeta.Namespace,
		Name:      pod.Spec.ServiceAccountName,
	}
	gsa, found := ns.verifiedSAs.get(ksa)
	if !found {
		klog.V(5).Infof("ServiceAccount %q is not authorized to act as any GSA.", ksa)
		return nil
	}
	node := pod.Spec.NodeName
	klog.Infof("Adding GSA %q to Node %q where Pod %q is running as KSA %q.", gsa, node, key, ksa)
	gsaLast, found := ns.nodes.add(node, pod.ObjectMeta.UID, key, gsa)
	if found && gsaLast == gsa {
		return nil
	}
	if found && gsaLast != gsa {
		klog.Infof("The authorized GSA of KSA %q that Pod %q runs as has has changed from %q to %q.", ksa, key, gsaLast, gsa)
	}
	return ns.sync(node)
}

// Sync calls HMS's SyncNode API to update the list of validate GSAs for a particular node.
func (ns *nodeSyncer) sync(node string) error {
	gsaList, err := ns.nodes.gsaEmailsByNode(node)
	if err != nil {
		return fmt.Errorf("Failed to retrieve GSA list for Node %q: %v", node, err)
	}
	return ns.hms.sync(node, ns.location, gsaList)
}

// podMap is a map of pods keyed by their UID
type podMap map[types.UID]pod

// pod contains the key of the pod from the informer events (ie, "<namespace>/<name>") and the GSA
// that the pod was authorized to run as.
type pod struct {
	key string
	gsa gsaEmail
}

// nodeMap is a thread-safe map of podMap's indexed by Node name.
type nodeMap struct {
	sync.RWMutex
	m map[string]podMap
}

func newNodeMap() *nodeMap {
	return &nodeMap{
		m: make(map[string]podMap),
	}
}

func (nm *nodeMap) add(node string, podUID types.UID, podKey string, gsa gsaEmail) (gsaEmail, bool) {
	nm.Lock()
	defer nm.Unlock()
	n, found := nm.m[node]
	if !found {
		p := pod{podKey, gsa}
		nm.m[node] = map[types.UID]pod{podUID: p}
		return "", false
	}
	p, found := n[podUID]
	n[podUID] = pod{podKey, gsa}
	return p.gsa, found
}

// Remove removes all the pods with matching podKey and returns the list of the removed pods' GSA
// per node in a map keyed by the node name.
func (nm *nodeMap) remove(podKey string) map[string][]gsaEmail {
	nm.Lock()
	defer nm.Unlock()

	rc := make(map[string][]gsaEmail)
	for node, pods := range nm.m {
		for podUID, pod := range pods {
			if pod.key == podKey {
				rc[node] = append(rc[node], pod.gsa)
				delete(pods, podUID)
			}
		}
	}
	return rc
}

func (nm *nodeMap) gsaEmailsByNode(node string) ([]gsaEmail, error) {
	nm.RLock()
	defer nm.RUnlock()
	if _, found := nm.m[node]; !found {
		return nil, fmt.Errorf("Node not found: %q", node)
	}
	set := make(map[gsaEmail]bool)
	for _, pod := range nm.m[node] {
		set[pod.gsa] = true
	}
	l := make([]gsaEmail, 0, len(set))
	for gsa := range set {
		l = append(l, gsa)
	}
	return l, nil
}
