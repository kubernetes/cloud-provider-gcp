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
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/tools/cache"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/controller"
)

const (
	nodeSyncerControlLoopName = "node-syncer"
	nodeSyncerQueueName       = "node-syncer-queue"
	nodeSyncerQueueRetryLimit = 5
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
	if !controller.WaitForCacheSync(nodeSyncerControlLoopName, stopCh, ns.hasSynced) {
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
		if ns.queue.NumRequeues(key) > nodeSyncerQueueRetryLimit {
			klog.Errorf("Stop retrying %q in queue; last error: %v", key, err)
			return true
		}
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
// In each nodeMap entry, the validated GSA is stored in a map indexed by the pod's workqueue key.
// This key (ie, <namespace name>/<pod name>) is chosen (over the Pod's UID) because it is the only
// piece of information available to this control-loop on pod delete events.
//
// TODO(danielywong): Handle pod recreate with the same name regardless of its Add/Delete event
// processing order.  Pod names are unique within a namespace but can be reused.  The processing
// order of Delete and Add events for a pod is therefore significant.  One possible resolution is by
// delay processing (ie, requeue) of Add event if the pod's UID have changed in order to give time
// for the Delete event to be processed.  A max requeue count will also be needed in case the Delete
// event was lost.
//
// TODO(danielywong): Call ns.sync only if necessary; that is, first use of a GSA on Pod addition or
// last use on Pod delete.
func (ns *nodeSyncer) process(key string) error {
	o, exists, err := ns.indexer.GetByKey(key)
	if err != nil {
		return fmt.Errorf("failed to get Pod %q: %v", key, err)
	}
	if !exists {
		node, gsa, found := ns.nodes.remove(key)
		if !found {
			return nil // Pod's ServiceAccount doesn't have a verified GSA
		}
		klog.Infof("Pod %q and its GSA %q is removed from Node %q", key, gsa, node)
		return ns.sync(node)
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
		klog.Infof("ServiceAccount %q is not authorized to act as any GSA.", ksa)
		return nil
	}
	node := pod.Spec.NodeName
	klog.Infof("Adding GSA %q to Node %q where Pod %q is running as KSA %q.", gsa, node, key, ksa)
	gsaLast, found := ns.nodes.add(node, key, gsa)
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

// podMap is a map of GSAs indexed by Pod keys.
type podMap map[string]gsaEmail

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

func (nm *nodeMap) add(node, pod string, gsa gsaEmail) (gsaEmail, bool) {
	nm.Lock()
	defer nm.Unlock()
	n, found := nm.m[node]
	if !found {
		nm.m[node] = map[string]gsaEmail{pod: gsa}
		return "", false
	}
	g, found := n[pod]
	n[pod] = gsa
	return g, found
}

func (nm *nodeMap) remove(pod string) (string, gsaEmail, bool) {
	nm.Lock()
	defer nm.Unlock()
	for node, pods := range nm.m {
		if gsa, ok := pods[pod]; ok {
			delete(pods, pod)
			return node, gsa, true
		}
	}
	return "", "", false
}

func (nm *nodeMap) gsaEmailsByNode(node string) ([]gsaEmail, error) {
	nm.RLock()
	defer nm.RUnlock()
	if _, found := nm.m[node]; !found {
		return nil, fmt.Errorf("Node not found: %q", node)
	}
	set := make(map[gsaEmail]bool)
	for _, gsa := range nm.m[node] {
		set[gsa] = true
	}
	l := make([]gsaEmail, 0, len(set))
	for gsa := range set {
		l = append(l, gsa)
	}
	return l, nil
}
