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

// Package eventhandler provides a common workqueue based event handler
package eventhandler

import (
	"context"
	"time"

	"github.com/google/uuid"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/cloud-provider-gcp/cmd/gcp-controller-manager/dpwi/ctxlog"
	"k8s.io/klog/v2"
)

// EventHandler is a common workqueue based event handler.
type EventHandler struct {
	queue   workqueue.RateLimitingInterface
	process func(ctx context.Context, key string) error
}

// InitEventHandler initializes an exponential backoff queue.
// It accepts a process function, which process a single event.
func (eh *EventHandler) InitEventHandler(name string, process func(context.Context, string) error) {
	eh.queue = workqueue.NewNamedRateLimitingQueue(workqueue.NewMaxOfRateLimiter(
		workqueue.NewItemExponentialFailureRateLimiter(200*time.Millisecond, 1000*time.Second),
	), name)
	eh.process = process
}

// ResourceEventHandler puts resource add/update/delete events to the workqueue.
func (eh *EventHandler) ResourceEventHandler() cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    eh.onObjectAdd,
		UpdateFunc: eh.onObjectUpdate,
		DeleteFunc: eh.onObjectDelete,
	}
}

func (eh *EventHandler) onObjectAdd(obj interface{}) {
	eh.Enqueue(obj)
}

func (eh *EventHandler) onObjectUpdate(obj, oldObj interface{}) {
	eh.Enqueue(obj)
}

func (eh *EventHandler) onObjectDelete(obj interface{}) {
	eh.Enqueue(obj)
}

// Enqueue puts an object event into the workerqueue.
func (eh *EventHandler) Enqueue(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		klog.Errorf("Internal error. Couldn't get key for Object %+v: %v", obj, err)
		return
	}
	eh.queue.Add(key)
}

// EnqueueKey directly puts a key into the workerqueue.
func (eh *EventHandler) EnqueueKey(key string) {
	eh.queue.Add(key)
}

// Run starts a number of workers to process events. The workers will
// continue to run and process items from the workqueue until context is done.
func (eh *EventHandler) Run(ctx context.Context, workers int) {
	for i := 0; i < workers; i++ {
		go wait.Until(func() {
			eh.work(ctx)
		}, time.Second, ctx.Done())
	}
	<-ctx.Done()
}

func (eh *EventHandler) work(ctx context.Context) {
	for eh.processNext(ctx) {
	}
}

func (eh *EventHandler) processNext(ctx context.Context) bool {
	key, quit := eh.queue.Get()
	if quit {
		return false
	}
	defer eh.queue.Done(key)

	ctx = context.WithValue(ctx, ctxlog.EventKey, key)
	ctx = context.WithValue(ctx, ctxlog.BackgroundIDKey, uuid.New())
	err := eh.process(ctx, key.(string))
	if err != nil {
		ctxlog.Warningf(ctx, "Re-queue due to %v", err)
		eh.queue.AddRateLimited(key)
		return true
	}
	eh.queue.Forget(key)
	return true
}
