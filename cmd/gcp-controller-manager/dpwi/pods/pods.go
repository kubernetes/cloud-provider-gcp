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

// Package pods listens to and process pod events. If the KSA of the pod
// has a verified non-empty GSA, it sends a node sync event.
package pods

import (
	"context"
	"fmt"
	"time"

	core "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/cloud-provider-gcp/cmd/gcp-controller-manager/dpwi/ctxlog"
	"k8s.io/cloud-provider-gcp/cmd/gcp-controller-manager/dpwi/eventhandler"
	"k8s.io/cloud-provider-gcp/cmd/gcp-controller-manager/dpwi/serviceaccounts"
)

const (
	handlerResyncPeriod = 30 * time.Minute
)

// Handler handles pod events.
type Handler struct {
	eventhandler.EventHandler
	podIndexer cache.Indexer
	verifier   verifier
	syncer     syncer
}

type syncer interface {
	EnqueueKey(key string)
}

type verifier interface {
	VerifiedGSA(ctx context.Context, ksa serviceaccounts.ServiceAccount) (serviceaccounts.GSAEmail, error)
}

// NewEventHandler creates a new pod handler.
func NewEventHandler(informer cache.SharedIndexInformer, verifier verifier, syncer syncer) (*Handler, error) {
	h := &Handler{
		podIndexer: informer.GetIndexer(),
		verifier:   verifier,
		syncer:     syncer,
	}

	h.InitEventHandler("pod-event", h.process)
	informer.AddEventHandlerWithResyncPeriod(h.ResourceEventHandler(), handlerResyncPeriod)
	return h, nil
}

// process() processes a pod event. If the KSA of the pod has a verified non-empty
// GSA, it sends a node sync event.
func (h *Handler) process(ctx context.Context, key string) error {
	o, exists, err := h.podIndexer.GetByKey(key)
	if err != nil {
		return fmt.Errorf("failed to get Pod %q: %w", key, err)
	}
	if !exists { // pod removal event
		// Do nothing. Wait for another pod event or re-sync.
		return nil
	}
	pod, ok := o.(*core.Pod)
	if !ok {
		return fmt.Errorf("invalid pod object from key %q: %#v", key, o)
	}

	ksa := serviceaccounts.ServiceAccount{
		Namespace: pod.ObjectMeta.Namespace,
		Name:      pod.Spec.ServiceAccountName,
	}
	gsa, err := h.verifier.VerifiedGSA(ctx, ksa)
	if err != nil {
		return fmt.Errorf("failed to get verified GSA for ksa %v: %w", ksa, err)
	}
	if gsa == "" {
		return nil
	}
	node := pod.Spec.NodeName
	if node == "" {
		return nil
	}
	h.syncer.EnqueueKey(node)
	ctxlog.Infof(ctx, "Processed pod with gsa %q on node %q", gsa, node)
	return nil
}
