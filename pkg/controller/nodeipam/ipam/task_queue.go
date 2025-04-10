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
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

// TaskQueue is a wrapper of workqueue that can have multiple workers processing queue items in parallel
type TaskQueue[T comparable] struct {
	// resource is used for logging to distinguish the queue being used.
	resource string
	// queue is the work queue the workers poll.
	queue workqueue.TypedRateLimitingInterface[T]
	// sync is called for each item in the queue.
	sync       func(item T) error
	workerDone []chan struct{}
	// numWorkers indicates the number of worker routines processing the queue.
	numWorkers int
}

// Run spawns off n parallel worker routines and returns immediately.
func (t *TaskQueue[T]) Run() {
	for worker := 0; worker < t.numWorkers; worker++ {
		klog.V(3).Infof("Spawning off worker for taskQueue workerId:%q resource:%q", worker, t.resource)
		go t.runInternal(worker)
	}
}

// runInternal invokes the worker routine to pick up and process an item from the queue. This blocks until ShutDown is called.
func (t *TaskQueue[T]) runInternal(workerID int) {
	for {
		item, quit := t.queue.Get()
		if quit {
			close(t.workerDone[workerID])
			return
		}
		klog.V(4).Infof("Syncing workerId %q item %v resource %q ", workerID, item, t.resource)
		if err := t.sync(item); err != nil {
			klog.Error(err, "Requeuing due to error", "workerId", workerID, "item", item, "resource", t.resource)
			t.queue.AddRateLimited(item)
		} else {
			klog.V(4).Infof("Finished syncing workerId:%q item:%v", workerID, item)
			t.queue.Forget(item)
		}
		t.queue.Done(item)
	}
}

func (t *TaskQueue[T]) Enqueue(item T) {
	klog.V(4).Infof("Enqueue object %q", t.resource)
	t.queue.Add(item)
}

// Shutdown shuts down the work queue and waits for the worker to ACK
func (t *TaskQueue[T]) Shutdown() {
	klog.V(2).Infof("Shutdown")
	t.queue.ShutDown()
	for _, workerDone := range t.workerDone {
		<-workerDone
	}
}

// NewTaskQueue creates a new task queue with the given sync function
// and rate limiter. The sync function is called for every element inserted into the queue.
func NewTaskQueue[T comparable](name, resource string, numWorkers int, syncFn func(T) error) *TaskQueue[T] {
	if numWorkers <= 0 {
		klog.V(3).Infof("Invalid worker count numWorkers:%q", numWorkers)
		return nil
	}
	rl := workqueue.DefaultTypedControllerRateLimiter[T]()
	var queue workqueue.TypedRateLimitingInterface[T]
	if name == "" {
		queue = workqueue.NewTypedRateLimitingQueue[T](rl)
	} else {
		queue = workqueue.NewTypedRateLimitingQueueWithConfig(rl, workqueue.TypedRateLimitingQueueConfig[T]{
			Name: name,
		})
	}

	taskQueue := &TaskQueue[T]{
		resource:   resource,
		queue:      queue,
		sync:       syncFn,
		numWorkers: numWorkers,
	}
	for worker := 0; worker < numWorkers; worker++ {
		taskQueue.workerDone = append(taskQueue.workerDone, make(chan struct{}))
	}
	return taskQueue
}
