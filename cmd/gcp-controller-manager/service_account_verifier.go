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
	"bytes"
	"context"
	"fmt"
	"time"

	compute "google.golang.org/api/compute/v1"

	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
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

	// verifiedSAConfigMapKey specifies the key to the ConfigMap's BinaryData map where the verified
	// KSA/GSA pairs are persisted in serialized form.
	verifiedSAConfigMapKey = "permitted-ksa-to-gsa-pairs"

	// serviceAccountAnnotationGSAEmail is the key to GCP Service Account annotation in
	// ServiceAccount objects.
	serviceAccountAnnotationGSAEmail = "iam.gke.io/gcp-service-account"

	// serviceAccountResyncPeriod defines the resync interval for the SA Informer.  This control
	// loop depends on this resync to pickup authorization configuration changes made in the GCP
	// side.  In other words, ServiceAccount configuration changes may take upto this resync period
	// to take effect.
	//
	// TODO(danielywong): make this a cmdline flag.
	serviceAccountResyncPeriod = 30 * time.Minute

	// configMapResyncPeriod defines the resync interval for the CM Informer.  This retry is mainly
	// an additional trigger in case of workqueue level retry exhaustion on write error.
	configMapResyncPeriod = 30 * time.Minute
)

// serviceAccountVerifier implements a custom control loop responsible for verifying authorization
// for Kubernetes Service Accounts (KSA) in the cluster to impersonate specific GCP Service Accounts
// (GSA) and persisting the authorized pairs in a ConfigMap entry.
//
// verifiedSAs is a thread-safe map of all authorized {KSA: GSA} pairs.  This control loop also
// maintains these pairs in ConfigMap "verifiedSAConfigMapNamespace/verifiedSAConfigMapName".
type serviceAccountVerifier struct {
	c           clientset.Interface
	saIndexer   cache.Indexer
	cmIndexer   cache.Indexer
	saHasSynced func() bool
	saQueue     workqueue.RateLimitingInterface
	cmQueue     workqueue.RateLimitingInterface
	verifiedSAs *saMap
	hms         *hmsClient
}

func newServiceAccountVerifier(client clientset.Interface, saInformer coreinformers.ServiceAccountInformer, cmInformer coreinformers.ConfigMapInformer, cs *compute.Service, sm *saMap, hmsAuthzURL string) (*serviceAccountVerifier, error) {
	hms, err := newHMSClient(hmsAuthzURL, &clientcmdapi.AuthProviderConfig{Name: "gcp"})
	if err != nil {
		return nil, err
	}
	sav := &serviceAccountVerifier{
		c:           client,
		saIndexer:   saInformer.Informer().GetIndexer(),
		cmIndexer:   cmInformer.Informer().GetIndexer(),
		saHasSynced: saInformer.Informer().HasSynced,
		saQueue: workqueue.NewNamedRateLimitingQueue(workqueue.NewMaxOfRateLimiter(
			workqueue.NewItemExponentialFailureRateLimiter(200*time.Millisecond, 1000*time.Second),
		), saVerifierSAQueueName),
		cmQueue: workqueue.NewNamedRateLimitingQueue(workqueue.NewMaxOfRateLimiter(
			workqueue.NewItemExponentialFailureRateLimiter(200*time.Millisecond, 1000*time.Second),
		), saVerifierCMQueueName),
		verifiedSAs: sm,
		hms:         hms,
	}
	saInformer.Informer().AddEventHandlerWithResyncPeriod(cache.ResourceEventHandlerFuncs{
		AddFunc:    sav.onSAAdd,
		UpdateFunc: sav.onSAUpdate,
		DeleteFunc: sav.onSADelete,
	}, serviceAccountResyncPeriod)
	cmInformer.Informer().AddEventHandlerWithResyncPeriod(cache.ResourceEventHandlerFuncs{
		AddFunc:    sav.onCMAdd,
		UpdateFunc: sav.onCMUpdate,
		DeleteFunc: sav.onCMDelete,
	}, configMapResyncPeriod)
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
		klog.Errorf("internal error. Couldn't get key for ServiceAccount %+v: %v", obj, err)
		return
	}
	sav.saQueue.AddRateLimited(key)
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
		klog.Errorf("internal error. Couldn't get key for ConfigMap %+v: %v", obj, err)
		return
	}
	sav.cmQueue.AddRateLimited(key)
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
	if !cache.WaitForNamedCacheSync(saVerifierControlLoopName, stopCh, sav.saHasSynced) {
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
		sav.saQueue.AddRateLimited(key)
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
	o, exists, err := sav.saIndexer.GetByKey(key)
	if err != nil {
		return false, fmt.Errorf("failed to get ServiceAccount %q: %v", key, err)
	}
	if !exists {
		// Remove the ksa entry from verifiedSAs in case it was previosly authorized.
		namespace, name, err := cache.SplitMetaNamespaceKey(key)
		if err != nil {
			klog.Errorf("Dropping invalid key %q in SA queue: %v", key, err)
			return false, nil
		}
		ksa := serviceAccount{namespace, name}
		if removedGSA, found := sav.verifiedSAs.remove(ksa); found {
			klog.Infof("Removed permission %q:%q; KSA removed", ksa, removedGSA)
			return true, nil
		}
		return false, nil
	}
	sa, ok := o.(*core.ServiceAccount)
	if !ok {
		klog.Errorf("Dropping invalid object from SA queue with key %q: %#v", key, o)
		return false, nil
	}
	ksa := serviceAccount{sa.ObjectMeta.Namespace, sa.ObjectMeta.Name}

	ann, found := sa.ObjectMeta.Annotations[serviceAccountAnnotationGSAEmail]
	if !found || ann == "" {
		// Annotation added (by admin) will not take effect until the SA's next periodic resync.
		klog.V(5).Infof("SA %v does not have a GsaEmail annotation.", sa)
		if removedGSA, found := sav.verifiedSAs.remove(ksa); found {
			klog.Infof("Removed permission %q:%q; annotation removed", ksa, removedGSA)
			return true, nil
		}
		return false, nil
	}
	gsa := gsaEmail(ann)
	permitted, err := sav.hms.authorize(ksa, gsa)
	if err != nil {
		return false, fmt.Errorf("failed to authorize %s:%s; err: %v", ksa, gsa, err)
	}

	if !permitted {
		if removedGSA, found := sav.verifiedSAs.remove(ksa); found {
			if removedGSA == gsa {
				klog.Infof("Removed permission %q:%q; no longer valid", ksa, gsa)
			} else {
				klog.Infof("Removed permission %q:%q; current annotation :%q is denied", ksa, removedGSA, gsa)
			}
			// Trigger CM update if SA was found (ie, previously permitted)
			return true, nil
		}
		klog.Infof("Permission denied %q:%q", ksa, gsa)
		return false, nil
	}
	previousGSA, found := sav.verifiedSAs.add(ksa, gsa)
	if !found {
		klog.Infof("Permission verified %q:%q", ksa, gsa)
		return true, nil
	} else if previousGSA != gsa {
		klog.Infof("Permission changed to %q:%q from :%q", ksa, gsa, previousGSA)
		return true, nil
	}
	klog.Infof("Permission re-verified %q:%q", ksa, gsa)
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

	o, exists, err := sav.cmIndexer.GetByKey(key)
	if err != nil {
		return fmt.Errorf("failed to get ConfigMap %q: %v", key, err)
	}
	if !exists {
		klog.Warningf("ConfigMap %s/%s does not exist; creating.", namespace, name)
		text, err := sav.verifiedSAs.serialize()
		if err != nil {
			return fmt.Errorf("internal error during serialization: %v", err)
		}
		cm := newVerifiedSAConfigMap(text)
		klog.V(5).Infof("Creating ConfigMap: %+v", cm.Data)
		_, err = sav.c.CoreV1().ConfigMaps(verifiedSAConfigMapNamespace).Create(context.TODO(), cm, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create ConfigMap: %v", err)
		}
		return nil
	}

	cm, ok := o.(*core.ConfigMap)
	if !ok {
		klog.Errorf("Dropping invalid object from ConfigMap queue with key %q: %#v", key, o)
		return nil
	}
	text, err := sav.verifiedSAs.serialize()
	if err != nil {
		return fmt.Errorf("internal error during serialization: %v", err)
	}
	if cm.BinaryData == nil {
		cm.BinaryData = make(map[string][]byte)
	}
	if b, found := cm.BinaryData[verifiedSAConfigMapKey]; found && bytes.Equal(text, b) {
		klog.V(5).Infof("ConfigMap in sync; no update necessary: %+v", cm.BinaryData)
		return nil
	}
	cm.BinaryData[verifiedSAConfigMapKey] = text
	klog.V(5).Infof("Updating ConfigMap: %+v", cm.Data)
	_, err = sav.c.CoreV1().ConfigMaps(verifiedSAConfigMapNamespace).Update(context.TODO(), cm, metav1.UpdateOptions{})
	if err != nil {
		// Fail-close by deleting the ConfigMap assuming update failure was due to invalid content.
		// Retries are triggered at workqueue level (subject to verfiiedCMQueueRetryLimit), any CM
		// or SA update, and CM Informer level periodic resync.
		//
		// TODO(danielywong): catch TooLong error returned from validation.ValidateConfigMap for
		// alerting.
		rmErr := sav.c.CoreV1().ConfigMaps(verifiedSAConfigMapNamespace).Delete(context.TODO(), key, *metav1.NewDeleteOptions(0))
		if rmErr != nil {
			return fmt.Errorf("failed to update ConfigMap (%v) and reset also failed (%v)", err, rmErr)
		}
		return fmt.Errorf("reset ConfigMap due to update error: %v", err)
	}
	return nil
}

func newEmptyVerifiedSAConfigMap() *core.ConfigMap {
	return newVerifiedSAConfigMap(nil)
}

func newVerifiedSAConfigMap(v []byte) *core.ConfigMap {
	return &core.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: verifiedSAConfigMapNamespace,
			Name:      verifiedSAConfigMapName,
		},
		BinaryData: map[string][]byte{verifiedSAConfigMapKey: v},
	}
}
