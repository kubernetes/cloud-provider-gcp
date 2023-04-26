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

package configmap

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coreinformers "k8s.io/client-go/informers/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/cloud-provider-gcp/cmd/gcp-controller-manager/dpwi/ctxlog"
	"k8s.io/cloud-provider-gcp/cmd/gcp-controller-manager/dpwi/eventhandler"
	"k8s.io/cloud-provider-gcp/cmd/gcp-controller-manager/dpwi/serviceaccounts"
)

const (
	// configMapQueueName is the name of the configMap object workqueue.
	configMapQueueName = "configmap-queue"

	// verifiedSAConfigMapNamespace specifies the namespace of the ConfigMap object
	// to persist the verified SA pairs.
	verifiedSAConfigMapNamespace = "kube-system"

	// verifiedSAConfigMapName specifies the name of the ConfigMap object
	// to persist the verified SA pairs.
	verifiedSAConfigMapName = "verified-ksa-to-gsa"

	// verifiedSAConfigMapKey specifies the key to the ConfigMap's BinaryData map where the verified
	// KSA/GSA pairs are persisted in serialized form.
	verifiedSAConfigMapKey = "permitted-ksa-to-gsa-pairs"

	configMapResyncPeriod = 30 * time.Minute
)

var (
	verifiedSAConfigMapCacheKey = fmt.Sprintf("%s/%s", verifiedSAConfigMapNamespace, verifiedSAConfigMapName)
)

// Handler listens to and process config map events.
// It gets all verified KSA->GSA pairs from the service account verifier.
// It creates/updates the in-cluster config map if they are different.
type Handler struct {
	eventhandler.EventHandler
	c         clientset.Interface
	cmIndexer cache.Indexer
	verifier  verifier
}

type verifier interface {
	AllVerified(ctx context.Context) (map[serviceaccounts.ServiceAccount]serviceaccounts.GSAEmail, error)
}

// NewEventHandler creates a new event handler.
func NewEventHandler(client clientset.Interface, cmInformer coreinformers.ConfigMapInformer, verifier verifier) *Handler {
	h := &Handler{
		c:         client,
		cmIndexer: cmInformer.Informer().GetIndexer(),
		verifier:  verifier,
	}
	h.InitEventHandler(configMapQueueName, h.persist)
	cmInformer.Informer().AddEventHandlerWithResyncPeriod(h.ResourceEventHandler(), configMapResyncPeriod)
	return h
}

// Enqueue puts the key of the verifiedSA configMap into the queue.
func (h *Handler) Enqueue() {
	h.EnqueueKey(verifiedSAConfigMapCacheKey)
}

// Persist creates or update the in-cluster configMap with all the verified GSAs.
// It returns an error if persist should be scheduled for retry.
func (h *Handler) persist(ctx context.Context, key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	if namespace != verifiedSAConfigMapNamespace || name != verifiedSAConfigMapName {
		return nil
	}

	o, exists, err := h.cmIndexer.GetByKey(key)
	if err != nil {
		return fmt.Errorf("failed to get ConfigMap %q: %w", key, err)
	}
	verified, err := h.verifier.AllVerified(ctx)
	if err != nil {
		return err
	}
	text, err := serialize(verified)
	if err != nil {
		return err
	}

	if !exists {
		cm := newVerifiedSAConfigMap(text)
		_, err = h.c.CoreV1().ConfigMaps(verifiedSAConfigMapNamespace).Create(ctx, cm, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create ConfigMap: %w", err)
		}
		ctxlog.Infof(ctx, "Created ConfigMap with size %v", len(verified))
		return nil
	}

	cm, ok := o.(*core.ConfigMap)
	if !ok {
		return fmt.Errorf("invalid object from ConfigMap queue with key %q: %#v", key, o)
	}
	if cm.BinaryData == nil {
		cm.BinaryData = make(map[string][]byte)
	}
	if b, found := cm.BinaryData[verifiedSAConfigMapKey]; found && bytes.Equal(text, b) {
		ctxlog.Infof(ctx, "ConfigMap in sync; no update necessary")
		return nil
	}
	cm.BinaryData[verifiedSAConfigMapKey] = text
	_, err = h.c.CoreV1().ConfigMaps(verifiedSAConfigMapNamespace).Update(ctx, cm, metav1.UpdateOptions{})
	if err != nil {
		// Fail-close by deleting the ConfigMap assuming update failure was due to invalid content.
		// Retries are triggered at workqueue level (subject to verfiiedCMQueueRetryLimit), any CM
		// or SA update, and CM Informer level periodic resync.
		//
		// TODO(danielywong): catch TooLong error returned from validation.ValidateConfigMap for
		// alerting.
		rmErr := h.c.CoreV1().ConfigMaps(verifiedSAConfigMapNamespace).Delete(ctx, name, *metav1.NewDeleteOptions(0))
		if rmErr != nil {
			return fmt.Errorf("failed to update ConfigMap (%v) and reset also failed (%w)", err, rmErr)
		}
		return fmt.Errorf("recmt ConfigMap due to update error: %w", err)
	}
	ctxlog.Infof(ctx, "Updated Configmap with size %v", len(verified))
	return nil
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
func serialize(m map[serviceaccounts.ServiceAccount]serviceaccounts.GSAEmail) ([]byte, error) {
	return json.Marshal(m)
}
