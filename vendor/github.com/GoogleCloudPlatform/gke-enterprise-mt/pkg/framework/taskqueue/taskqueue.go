// Package taskqueue provides a task queue for syncing objects in parallel.
package taskqueue

import (
	"context"
	"fmt"

	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

var (
	// KeyFunc is the default key function for the task queue.
	// It uses the cache.DeletionHandlingMetaNamespaceKeyFunc, which is the same as the default
	// key function for the k8s workqueue.
	KeyFunc = cache.DeletionHandlingMetaNamespaceKeyFunc
)

// TaskQueue is a rate limited operation queue.
type TaskQueue interface {
	// Run starts the task queue.
	Run()
	// Enqueue adds one or more keys to the work queue.
	Enqueue(objs ...any)
	// Shutdown shuts down the work queue and waits for all the workers to ACK.
	Shutdown()
	// Len returns the length of the queue.
	Len() int
	// NumRequeues returns the number of times the given item was requeued.
	NumRequeues(obj any) int
	// ShuttingDown returns true if the queue is shutting down.
	ShuttingDown() bool
}

// PeriodicTaskQueueWithMultipleWorkers invokes the given sync function for every work item
// inserted, while running n parallel worker routines. If the sync() function results in an error, the item is put on
// the work queue after a rate-limit.
type PeriodicTaskQueueWithMultipleWorkers struct {
	// resource is used for logging to distinguish the queue being used.
	resource string
	// keyFunc translates an object to a string-based key.
	keyFunc func(obj any) (string, error)
	// queue is the work queue the workers poll.
	queue workqueue.RateLimitingInterface
	// sync is called for each item in the queue.
	sync func(context.Context, string) error
	// The respective workerDone channel is closed when the worker exits. There is one channel per worker.
	workerDone []chan struct{}
	// numWorkers indicates the number of worker routines processing the queue.
	numWorkers int
}

// Len returns the length of the queue.
func (t *PeriodicTaskQueueWithMultipleWorkers) Len() int {
	return t.queue.Len()
}

// NumRequeues returns the number of times the given item was requeued.
func (t *PeriodicTaskQueueWithMultipleWorkers) NumRequeues(obj any) int {
	key, err := t.keyFunc(obj)
	if err != nil {
		// This error is unexpected because Enqueue should prevent objects that fail keyFunc from being added.
		klog.Errorf("Couldn't get key for object when checking requeues: %v, objectType: %T, error: %v", fmt.Sprintf("%+v", obj), obj, err)
		return 0
	}
	return t.queue.NumRequeues(key)
}

// runInternal invokes the worker routine to pick up and process an item from the queue. This blocks until ShutDown is called.
func (t *PeriodicTaskQueueWithMultipleWorkers) runInternal(workerID int) {
	ctx := context.Background()
	for {
		key, quit := t.queue.Get()
		if quit {
			close(t.workerDone[workerID])
			return
		}
		klog.V(4).Info("Syncing", "workerId", workerID, "key", key, "resource", t.resource)
		if err := t.sync(ctx, key.(string)); err != nil {
			klog.Errorf("Requeuing due to error: %v, workerId: %v, key: %v, resource: %v", err, workerID, key, t.resource)
			t.queue.AddRateLimited(key)
		} else {
			klog.V(4).Info("Finished syncing", "workerId", workerID, "key", key)
			t.queue.Forget(key)
		}
		t.queue.Done(key)
	}
}

// Run spawns off n parallel worker routines and returns immediately.
func (t *PeriodicTaskQueueWithMultipleWorkers) Run() {
	for worker := 0; worker < t.numWorkers; worker++ {
		klog.Info("Spawning off worker for taskQueue", "workerId", worker, "resource", t.resource)
		go t.runInternal(worker)
	}
}

// Enqueue adds one or more keys to the work queue.
func (t *PeriodicTaskQueueWithMultipleWorkers) Enqueue(objs ...any) {
	for _, obj := range objs {
		key, err := t.keyFunc(obj)
		if err != nil {
			klog.Errorf("Couldn't get key for object: %v, objectType: %T, error: %v", fmt.Sprintf("%+v", obj), obj, err)
			return
		}
		klog.V(4).Info("Enqueue key", "key", key, "resource", t.resource)
		t.queue.Add(key)
	}
}

// Shutdown shuts down the work queue and waits for all the workers to ACK
func (t *PeriodicTaskQueueWithMultipleWorkers) Shutdown() {
	klog.V(2).Info("Shutting down task queue for resource", "resource", t.resource)
	t.queue.ShutDown()
	// wait for all workers to shutdown.
	for _, workerDone := range t.workerDone {
		<-workerDone
	}
}

// ShuttingDown returns true if the queue is shutting down.
func (t *PeriodicTaskQueueWithMultipleWorkers) ShuttingDown() bool {
	return t.queue.ShuttingDown()
}

// NewPeriodicTaskQueueWithMultipleWorkers creates a new task queue with the default rate limiter and the given number of worker goroutines.
func NewPeriodicTaskQueueWithMultipleWorkers(name, resource string, numWorkers int, syncFn func(context.Context, string) error) *PeriodicTaskQueueWithMultipleWorkers {
	if numWorkers <= 0 {
		klog.Errorf("Invalid worker count: %v", numWorkers)
		return nil
	}
	rl := workqueue.DefaultControllerRateLimiter()
	var queue workqueue.RateLimitingInterface
	if name == "" {
		queue = workqueue.NewRateLimitingQueue(rl)
	} else {
		queue = workqueue.NewNamedRateLimitingQueue(rl, name)
	}
	taskQueue := &PeriodicTaskQueueWithMultipleWorkers{
		resource:   resource,
		keyFunc:    KeyFunc,
		queue:      queue,
		sync:       syncFn,
		numWorkers: numWorkers,
	}
	for worker := 0; worker < numWorkers; worker++ {
		taskQueue.workerDone = append(taskQueue.workerDone, make(chan struct{}))
	}
	return taskQueue
}
