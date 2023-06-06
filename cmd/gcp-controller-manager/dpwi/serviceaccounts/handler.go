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

package serviceaccounts

import (
	"context"
	"fmt"
	"time"

	core "k8s.io/api/core/v1"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/cloud-provider-gcp/cmd/gcp-controller-manager/dpwi/ctxlog"
	"k8s.io/cloud-provider-gcp/cmd/gcp-controller-manager/dpwi/eventhandler"
)

const (
	saVerifierSAQueueName = "service-account-verifier-sa-queue"

	serviceAccountResyncPeriod = 30 * time.Minute

	indexByKSA = "index-by-ksa"
)

// Handler listens to and process Kubernetes Service Accounts (KSA) events.
// It forces the Verifier to verify if a KSA can act as a GCP Service Account (GSA) or not,
// and then update Verifier's in memory result. When a KSA's permission changes,
// it notifies the configmap handler to update the configmap and the node syncer
// to sync all related nodes.
type Handler struct {
	eventhandler.EventHandler
	podIndexer             cache.Indexer
	verifier               *Verifier
	notifyConfigmapHandler func()
	notifyNodeSyncer       func(string)
}

// NewEventHandler creates a new handler.
func NewEventHandler(
	saInformer coreinformers.ServiceAccountInformer,
	podInformer coreinformers.PodInformer,
	verifier *Verifier,
	notifyConfigmapHandler func(),
	notifyNodeSyncer func(string),
) *Handler {
	podIndexer := podInformer.Informer().GetIndexer()
	podIndexer.AddIndexers(map[string]cache.IndexFunc{
		indexByKSA: func(obj interface{}) ([]string, error) {
			pod, ok := obj.(*core.Pod)
			if !ok {
				return nil, fmt.Errorf("invalid pod object from key %q", obj)
			}
			ksa := fmt.Sprintf("%s/%s", pod.ObjectMeta.Namespace, pod.Spec.ServiceAccountName)
			return []string{ksa}, nil
		},
	})
	h := &Handler{
		podIndexer:             podIndexer,
		verifier:               verifier,
		notifyConfigmapHandler: notifyConfigmapHandler,
		notifyNodeSyncer:       notifyNodeSyncer,
	}
	h.InitEventHandler(saVerifierSAQueueName, h.process)
	saInformer.Informer().AddEventHandlerWithResyncPeriod(h.ResourceEventHandler(), serviceAccountResyncPeriod)
	return h
}

func (h *Handler) process(ctx context.Context, key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	ksa := ServiceAccount{Namespace: namespace, Name: name}
	res, err := h.verifier.ForceVerify(ctx, ksa)
	if err != nil {
		return err
	}
	if res.denied {
		// https://cloud.google.com/iam/docs/access-change-propagation
		return fmt.Errorf("retry denied error as IAM propagation can take 7 minutes or longer")
	}
	if res.curGSA == res.preVerifiedGSA {
		return nil
	}
	// gsa changes, so the permission changes.
	h.notifyConfigmapHandler()
	h.notifyAffectedNodes(ctx, ksa)
	return nil
}

func (h *Handler) notifyAffectedNodes(ctx context.Context, ksa ServiceAccount) {
	nodes := make(map[string]bool)
	pods, err := h.podIndexer.ByIndex(indexByKSA, ksa.Key())
	if err != nil {
		ctxlog.Warningf(ctx, "Failed to get pods for ksa %q: %v", ksa.Key(), err)
		return
	}
	for _, o := range pods {
		pod, ok := o.(*core.Pod)
		if !ok {
			ctxlog.Warningf(ctx, "invalid pod object from podIndexer %#v", o)
			continue
		}

		if ksa.Namespace != pod.ObjectMeta.Namespace ||
			ksa.Name != pod.Spec.ServiceAccountName {
			continue
		}
		nodes[pod.Spec.NodeName] = true
	}
	var li []string
	for node := range nodes {
		h.notifyNodeSyncer(node)
		li = append(li, node)
	}
	ctxlog.Infof(ctx, "KSA permission change, notify nodes: %+q", li)
}
