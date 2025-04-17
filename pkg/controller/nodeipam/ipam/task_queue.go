/*
Copyright 2025 The Kubernetes Authors.

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
	"sync"

	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

// TaskQueue is a wrapper of workqueue that can have multiple workers processing queue items in parallel
type TaskQueue struct {
	// resource is used for logging to distinguish the queue being used.
	resource string
	// queue is the work queue the workers poll.
	queue workqueue.TypedRateLimitingInterface[string]
	// keyFunc translates an object to a string-based key.
	keyFunc func(obj interface{}) (string, error)
	// sync is called for each item in the queue.
	sync func(item string) error
	// wait for multiple workers to finish
	wg sync.WaitGroup
	// numWorkers indicates the number of worker routines processing the queue.
	numWorkers int
}

// Run spawns off n parallel worker routines and returns immediately.
func (t *TaskQueue) Run() {
	for worker := 0; worker < t.numWorkers; worker++ {
		klog.InfoS("Spawning off worker for taskQueue", "workerId", worker, "resource", t.resource)
		t.wg.Add(1)
		go func() {
			t.runInternal(worker)
		}()
	}
}

// runInternal invokes the worker routine to pick up and process an item from the queue. This blocks until ShutDown is called.
func (t *TaskQueue) runInternal(workerID int) {
	for {
		item, quit := t.queue.Get()
		if quit {
			t.wg.Done()
			return
		}
		klog.InfoS("Syncing worker item", "workerID", workerID, "item", item, "resource", t.resource)
		if err := t.sync(item); err != nil {
			klog.ErrorS(err, "Requeuing due to error", "workerId", workerID, "item", item, "resource", t.resource)
			t.queue.AddRateLimited(item)
		} else {
			klog.InfoS("Finished syncing", "workderID", workerID, "item", item)
			t.queue.Forget(item)
		}
		t.queue.Done(item)
	}
}

// Enqueue adds a new item to the queue.
func (t *TaskQueue) Enqueue(obj interface{}) {
	key, err := t.keyFunc(obj)
	if err != nil {
		klog.ErrorS(err, "Couldn't get object key", "object", obj, "resource", t.resource)
		return
	}
	klog.InfoS("Enqueue object", "object", obj, "resource", t.resource)
	t.queue.Add(key)
}

// Shutdown shuts down the work queue and waits for the worker to ACK
func (t *TaskQueue) Shutdown() {
	klog.InfoS("Shutdown")
	t.queue.ShutDown()
	t.wg.Wait()
}

// NewTaskQueue creates a new task queue with the given sync function
// and rate limiter. The sync function is called for every element inserted into the queue.
func NewTaskQueue(name, resource string, numWorkers int, keyFn func(obj interface{}) (string, error), syncFn func(string) error) *TaskQueue {
	if numWorkers <= 0 {
		klog.InfoS("Invalid worker count", "numWorkers", numWorkers)
		return nil
	}
	rl := workqueue.DefaultTypedControllerRateLimiter[string]()
	var queue workqueue.TypedRateLimitingInterface[string]
	if name == "" {
		queue = workqueue.NewTypedRateLimitingQueue[string](rl)
	} else {
		queue = workqueue.NewTypedRateLimitingQueueWithConfig(rl, workqueue.TypedRateLimitingQueueConfig[string]{
			Name: name,
		})
	}

	taskQueue := &TaskQueue{
		resource:   resource,
		queue:      queue,
		keyFunc:    keyFn,
		sync:       syncFn,
		numWorkers: numWorkers,
		wg:         sync.WaitGroup{},
	}
	return taskQueue
}
