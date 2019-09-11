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
	"time"

	compute "google.golang.org/api/compute/v1"

	"github.com/google/go-cmp/cmp"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/controller"
)

const (
	// saVerifierControlLoopName is the name of this control loop.
	saVerifierControlLoopName = "service-account-verifier"

	// saVerifierSAQueueName is the name of the ServiceAccount object workqueue.
	saVerifierSAQueueName = "service-account-verifier-sa-queue"

	// saVerifierCMQueueName is the name of the ConfigMap object workqueue.
	saVerifierCMQueueName = "service-account-verifier-cm-queue"

	// saVerifiedSAQueueRetryLimit is the maximum number of back-to-back requeuing of a SA key.
	saVerifiedSAQueueRetryLimit = 5

	// saVerifiedCMQueueRetryLimit is the maximum number of back-to-back requeuing of a CM key.
	saVerifiedCMQueueRetryLimit = 5

	// verifiedSAConfigMapNamespace specifies the namespace of the ConfigMap object that this
	// control loop uses to persist the verified SA pairs.
	verifiedSAConfigMapNamespace = "kube-system"

	// verifiedSAConfigMapName specifies the name of the ConfigMap object that this control loop
	// uses to persist the verified SA pairs.
	verifiedSAConfigMapName = "verified-ksa-to-gsa"

	// serviceAccountAnnotationGsaEmail is the key to GCP Service Account annotation in
	// ServiceAccount objects.
	serviceAccountAnnotationGsaEmail = "iam.gke.io/gcp-service-account"

	// serviceAccountResyncPeriod defines the resync interval for the SA Informer.  This control
	// loop depends on this resync to pickup authorization configuration changes made in the GCP
	// side.  In other words, ServiceAccount configuration changes may take upto this resync period
	// to take effect.
	//
	// TODO(danielywong): make this a cmdline flag.
	serviceAccountResyncPeriod = 30 * time.Minute
)

// serviceAccountVerifier implements a custom control loop responsible for verifying authorization
// for Kubernetes Service Accounts (KSA) in the cluster to impersonate specific GCP Service Accounts
// (GSA) and persisting the authorized pairs in a ConfigMap entry.
//
// See go/gke-direct-path-controller-dd for details.
//
// verifiedSAs is a thread-safe map of all authorized {KSA: GSA} pairs.  This control loop also
// maintains these pairs in ConfigMap "verifiedSAConfigMapNamespace/verifiedSAConfigMapName".
type serviceAccountVerifier struct {
	c           clientset.Interface
	sals        corelisters.ServiceAccountLister
	cmls        corelisters.ConfigMapLister
	saHasSynced func() bool
	saQueue     workqueue.RateLimitingInterface
	cmQueue     workqueue.RateLimitingInterface
	verifiedSAs *saMap
}

func newServiceAccountVerifier(client clientset.Interface, saInformer coreinformers.ServiceAccountInformer, cmInformer coreinformers.ConfigMapInformer, cs *compute.Service, sm *saMap) (*serviceAccountVerifier, error) {
	sav := &serviceAccountVerifier{
		c:           client,
		sals:        saInformer.Lister(),
		cmls:        cmInformer.Lister(),
		saHasSynced: saInformer.Informer().HasSynced,
		saQueue: workqueue.NewNamedRateLimitingQueue(workqueue.NewMaxOfRateLimiter(
			workqueue.NewItemExponentialFailureRateLimiter(200*time.Millisecond, 1000*time.Second),
		), saVerifierSAQueueName),
		cmQueue: workqueue.NewNamedRateLimitingQueue(workqueue.NewMaxOfRateLimiter(
			workqueue.NewItemExponentialFailureRateLimiter(200*time.Millisecond, 1000*time.Second),
		), saVerifierCMQueueName),
		verifiedSAs: sm,
	}
	saInformer.Informer().AddEventHandlerWithResyncPeriod(cache.ResourceEventHandlerFuncs{
		AddFunc:    sav.onSAAdd,
		UpdateFunc: sav.onSAUpdate,
		DeleteFunc: sav.onSADelete,
	}, serviceAccountResyncPeriod)
	cmInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    sav.onCMAdd,
		UpdateFunc: sav.onCMUpdate,
		DeleteFunc: sav.onCMDelete,
	})
	return sav, nil
}

func (sav *serviceAccountVerifier) onSAAdd(obj interface{}) {
	klog.V(5).Infof("onSAAdd: %v", obj)
	sav.enqueueSA(obj)
}

func (sav *serviceAccountVerifier) onSAUpdate(obj, oldObj interface{}) {
	klog.V(5).Infof("onSAUpdate: %v -> %v", oldObj, obj)
	// Enqueues for verification even if no change because of periodic resync.
	// TODO(danielywong): confirm this is called upon resync
	sav.enqueueSA(obj)
}

func (sav *serviceAccountVerifier) onSADelete(obj interface{}) {
	klog.V(5).Infof("onSADelete: %v", obj)
	// TODO(danielywong): check delete behavior; expect obj still exist in cache store
	sav.enqueueSA(obj)
}

func (sav *serviceAccountVerifier) enqueueSA(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("internal error. Couldn't get key for ServiceAccount %+v: %v", obj, err))
		return
	}
	sav.saQueue.Add(key)
}

func (sav *serviceAccountVerifier) onCMAdd(obj interface{}) {
	// TODO(danielywong): check upon init this will be called and trigger the ConfigMap entry to be
	// reset.
	klog.V(5).Infof("onCMAdd: %v", obj)
	sav.enqueueCM(obj)
}

func (sav *serviceAccountVerifier) onCMUpdate(obj, oldObj interface{}) {
	klog.V(5).Infof("onCMUpdate: %v -> %v", oldObj, obj)
	sav.enqueueCM(obj)
}

func (sav *serviceAccountVerifier) onCMDelete(obj interface{}) {
	klog.V(5).Infof("onCMDelete: %v", obj)
	sav.enqueueCM(obj)
}

func (sav *serviceAccountVerifier) enqueueCM(obj interface{}) {
	cm, ok := obj.(*core.ConfigMap)
	if !ok || cm.ObjectMeta.Namespace != verifiedSAConfigMapNamespace || cm.ObjectMeta.Name != verifiedSAConfigMapName {
		return
	}
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("internal error. Couldn't get key for ConfigMap %+v: %v", obj, err))
		return
	}
	sav.cmQueue.Add(key)
}

// Run starts saWorkers number of ServiceAccount workers plus 1 ConfigMap worker to process this
// controller's two independent workqueues of the corresponding object types.  All these workers
// will continue to run and process items from their respective workqueues until receiving stopCh.
func (sav *serviceAccountVerifier) Run(saWorkers int, stopCh <-chan struct{}) {
	// Start SA queue processing upon initialization but block CM queue until this controller has
	// been informed of all SA objects (ie, saHasSynced).
	klog.V(5).Infof("%s starts SA processing ...", saVerifierControlLoopName)
	for i := 0; i < saWorkers; i++ {
		go wait.Until(sav.workSAQueue, time.Second, stopCh)
	}
	if !controller.WaitForCacheSync(saVerifierControlLoopName, stopCh, sav.saHasSynced) {
		return
	}
	klog.V(5).Infof("%s starts CM processing ...", saVerifierControlLoopName)
	go wait.Until(sav.workCMQueue, time.Second, stopCh)
	<-stopCh
	klog.V(5).Infof("%s has terminated.", saVerifierControlLoopName)
}

func (sav *serviceAccountVerifier) workSAQueue() {
	for sav.processNextSA() {
	}
}

func (sav *serviceAccountVerifier) processNextSA() bool {
	key, quit := sav.saQueue.Get()
	if quit {
		return false
	}
	defer sav.saQueue.Done(key)

	resyncCM, err := sav.verify(key.(string))
	if err != nil {
		if sav.saQueue.NumRequeues(key) > saVerifiedSAQueueRetryLimit {
			klog.Errorf("Stop retrying %q in SA queue; last error: %v", key, err)
			return true
		}
		klog.Warningf("Requeuing SA %q due to %v", key, err)
		sav.cmQueue.AddRateLimited(key)
		return true
	}
	if resyncCM {
		sav.addCMUpdate()
	}
	sav.saQueue.Forget(key)
	return true
}

// Verify verifies if the ServiceAccount identified by key is permitted to get certificates as the
// GSA as annotated.  Verify returns a bool to indicate if ConfigMap sync is required and an error
// if key needs to be requeued.
func (sav *serviceAccountVerifier) verify(key string) (bool, error) {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		klog.Errorf("Dropping invalid key %q in SA queue: %v", key, err)
		return false, nil
	}
	sa, err := sav.sals.ServiceAccounts(namespace).Get(name)
	if err != nil {
		return false, fmt.Errorf("failed to get ServiceAccount %q: %v", key, err)
	}
	ann, found := sa.ObjectMeta.Annotations[serviceAccountAnnotationGsaEmail]
	if !found {
		// Annotation added later (by admin) will not be picked up until the SA's next periodic
		// resync.
		klog.V(5).Infof("SA %v does not have a GsaEmail annotation.", sa)
		return false, nil
	}
	gsa := gsaEmail(ann)
	ksa := sa.ObjectMeta.Name
	kns := sa.ObjectMeta.Namespace
	klog.V(5).Infof("authorizing %s/%s:%s", kns, ksa, gsa)

	// Authorize the (SA,GSA) pair and update verifiedSAs accordingly.
	permitted := true // TODO(danielywong): call hms::AuthorizeSAMapping to validate permission.

	if !permitted {
		klog.V(5).Infof("not permitted %s/%s:%s", kns, ksa, gsa)
		if removedGSA, found := sav.verifiedSAs.remove(serviceAccount{kns, ksa}); found {
			if removedGSA == gsa {
				klog.V(5).Infof("removed %s/%s:%s which is no longer permitted", kns, ksa, gsa)
			} else {
				klog.V(5).Infof("removed %s/%s:%s due to new annotation %s which is not permitted", kns, ksa, removedGSA, gsa)
			}
			// Trigger CM update if SA was found (ie, previously permitted)
			return true, nil
		}
		return false, nil
	}
	klog.V(5).Infof("permitted %s/%s:%s", kns, ksa, gsa)
	previousGSA, found := sav.verifiedSAs.add(serviceAccount{kns, ksa}, gsa)
	if !found {
		klog.V(5).Infof("added %s/%s:%s", kns, ksa, gsa)
		return true, nil
	} else if previousGSA != gsa {
		klog.V(5).Infof("updated %s/%s:%s from :%s", kns, ksa, gsa, previousGSA)
		return true, nil
	}
	return false, nil
}

func (sav *serviceAccountVerifier) addCMUpdate() {
	key, err := cache.MetaNamespaceKeyFunc(newEmptyVerifiedSAConfigMap())
	if err != nil {
		klog.Errorf("Internal error. Couldn't get key for empty ConfigMap: %v", err)
		return
	}
	sav.cmQueue.Add(key)
}

func (sav *serviceAccountVerifier) workCMQueue() {
	for sav.processNextCM() {
	}
}

func (sav *serviceAccountVerifier) processNextCM() bool {
	key, quit := sav.cmQueue.Get()
	if quit {
		return false
	}
	defer sav.cmQueue.Done(key)

	if err := sav.persist(key.(string)); err != nil {
		if sav.cmQueue.NumRequeues(key) > saVerifiedCMQueueRetryLimit {
			klog.Errorf("Stop retrying %q in CM queue; last error: %v", key, err)
			return true
		}
		klog.Warningf("Requeuing CM %q due to %v", key, err)
		sav.cmQueue.AddRateLimited(key)
		return true
	}
	sav.cmQueue.Forget(key)
	return true
}

// Persist checks and persists sav.verifiedSAs in the ConfigMap identified by key if they are
// out of sync.  It returns an error if persist should be scheduled for retry.
func (sav *serviceAccountVerifier) persist(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		klog.Errorf("Dropping invalid key %q in CM queue: %v", key, err)
		return nil
	}
	if namespace != verifiedSAConfigMapNamespace || name != verifiedSAConfigMapName {
		klog.Errorf("Dropping unknown ConfigMap object %s/%s in CM queue.", namespace, name)
		return nil
	}

	cm, err := sav.cmls.ConfigMaps(namespace).Get(name)
	if err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("unknown error in getting ConfigMap %s/%s: %v", namespace, name, err)
		}
		klog.Warningf("ConfigMap %s/%s not found; creating.", namespace, name)
		cm = newEmptyVerifiedSAConfigMap()
		cm.Data = sav.verifiedSAs.stringMap()
		klog.V(5).Infof("Creating ConfigMap: %+v", cm.Data)
		_, err = sav.c.CoreV1().ConfigMaps(verifiedSAConfigMapNamespace).Create(cm)
		if err != nil {
			return fmt.Errorf("failed to create ConfigMap: %v", err)
		}
		return nil
	}

	sm := sav.verifiedSAs.stringMap()
	if cmp.Equal(sm, cm.Data) {
		klog.V(5).Infof("ConfigMap in sync; no update necessary: %+v", cm.Data)
		return nil
	}
	cm.Data = sm
	klog.V(5).Infof("Updating ConfigMap: %+v", cm.Data)
	_, err = sav.c.CoreV1().ConfigMaps(verifiedSAConfigMapNamespace).Update(cm)
	if err != nil {
		return fmt.Errorf("failed to update ConfigMap: %v", err)
	}
	return nil
}

func newEmptyVerifiedSAConfigMap() *core.ConfigMap {
	return &core.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: verifiedSAConfigMapNamespace,
			Name:      verifiedSAConfigMapName,
		},
	}
}
