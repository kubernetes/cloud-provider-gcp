// Package framework provides a generic controller implementation for managing
// the lifecycle of controllers that are scoped to ProviderConfig resources.
//
// It handles watching ProviderConfig resources, ensuring finalizers are present,
// and starting/stopping the associated controllers using a provided
// ControllerStarter implementation.
package framework

import (
	"context"
	"fmt"
	"math/rand"
	"runtime/debug"

	providerconfigv1 "github.com/GoogleCloudPlatform/gke-enterprise-mt/apis/providerconfig/v1"
	"github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/framework/mtcontext"
	"github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/framework/taskqueue"
	crv1 "github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/providerconfigcr"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

// ControllerStarter defines the interface for starting a controller for a ProviderConfig.
// Implementations encapsulate all controller-specific startup logic and dependencies.
type ControllerStarter interface {
	// StartController starts controller(s) for the given ProviderConfig.
	// Returns:
	//   - A channel that should be closed to stop the controller
	//   - An error if startup fails
	//
	// The returned stop channel will be closed by the framework when the
	// ProviderConfig is deleted or the controller needs to shut down.
	StartController(pc *providerconfigv1.ProviderConfig) (chan<- struct{}, error)
}

const (
	providerConfigControllerName = "provider-config-controller"
	resourceName                 = "provider-configs"
	workersCount                 = 5
)

// controllerManager implements the logic for starting and stopping controllers for each ProviderConfig.
type controllerManager interface {
	StartControllersForProviderConfig(ctx context.Context, pc *providerconfigv1.ProviderConfig) error
	StopControllersForProviderConfig(ctx context.Context, pc *providerconfigv1.ProviderConfig) error
}

// Controller manages the ProviderConfig resource lifecycle.
// It watches for ProviderConfig changes and delegates to the manager to start/stop
// controllers for each ProviderConfig.
type Controller struct {
	manager controllerManager

	providerConfigLister cache.Indexer
	providerConfigQueue  taskqueue.TaskQueue
	workersCount         int
	stopCh               <-chan struct{}
	hasSynced            func() bool
}

// New creates a new Controller that manages ProviderConfig resources.
func New(client dynamic.Interface, providerConfigInformer cache.SharedIndexInformer, finalizerName string, controllerStarter ControllerStarter, stopCh <-chan struct{},
) *Controller {
	manager := newManager(
		client,
		finalizerName,
		controllerStarter,
	)
	return newController(manager, providerConfigInformer, stopCh)
}

// newController creates a Controller with the given manager. Used for testing.
func newController(manager controllerManager, providerConfigInformer cache.SharedIndexInformer, stopCh <-chan struct{}) *Controller {
	c := &Controller{
		providerConfigLister: providerConfigInformer.GetIndexer(),
		stopCh:               stopCh,
		workersCount:         workersCount,
		hasSynced:            providerConfigInformer.HasSynced,
		manager:              manager,
	}

	c.providerConfigQueue = taskqueue.NewPeriodicTaskQueueWithMultipleWorkers(providerConfigControllerName, resourceName, c.workersCount, c.syncWrapper)

	providerConfigInformer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj any) {
				klog.V(4).Info("Enqueue add event", "object", obj)
				c.providerConfigQueue.Enqueue(obj)
			},
			UpdateFunc: func(old, cur any) {
				klog.V(4).Info("Enqueue update event", "old", old, "new", cur)
				c.providerConfigQueue.Enqueue(cur)
			},
		})

	klog.Info("ProviderConfig controller created")
	return c
}

// Run starts the controller and blocks until the stop channel is closed.
func (c *Controller) Run() {
	defer c.shutdown()

	klog.Info("Starting ProviderConfig controller")

	klog.Info("Waiting for initial cache sync before starting ProviderConfig Controller")
	ok := cache.WaitForCacheSync(c.stopCh, c.hasSynced)
	if !ok {
		klog.Error("Failed to wait for initial cache sync before starting ProviderConfig Controller")
		return
	}

	klog.Info("Started ProviderConfig Controller", "numWorkers", c.workersCount)
	c.providerConfigQueue.Run()

	<-c.stopCh
	klog.Info("ProviderConfig Controller exited")
}

func (c *Controller) shutdown() {
	klog.Info("Shutting down ProviderConfig Controller")
	c.providerConfigQueue.Shutdown()
}

func (c *Controller) syncWrapper(ctx context.Context, key string) (err error) {
	syncID := rand.Int31()

	defer func() {
		if r := recover(); r != nil {
			stack := string(debug.Stack())
	ctxErrorf(ctx, "panic in ProviderConfig sync worker goroutine: %v, stack: %s, syncId: %d", r, stack, syncID)
			err = fmt.Errorf("panic in sync worker: %v", r)
		}
	}()
	err = c.sync(ctx, key, syncID)
	if err != nil {
		ctxErrorf(ctx, "Error syncing providerConfig key: %s, syncId: %d, err: %v", key, syncID, err)
	}
	return err
}

func (c *Controller) sync(ctx context.Context, key string, syncID int32) error {
	obj, exists, err := c.providerConfigLister.GetByKey(key)
	if err != nil {
		return fmt.Errorf("failed to lookup providerConfig for key %s: %w", key, err)
	}
	if !exists || obj == nil {
		ctxInfoDepth(ctx, 3, fmt.Sprintf("ProviderConfig does not exist anymore (syncId: %d)", syncID))
		return nil
	}

	pc, err := crv1.NewProviderConfig(obj)
	if err != nil {
		return fmt.Errorf("failed to convert object to ProviderConfig: %w", err)
	}

	// Populate tenant context
	ctx = mtcontext.ContextWithTenantUID(ctx, pc.Name)
	logger := klog.FromContext(ctx).WithValues("tenantUID", pc.Name)
  ctx = klog.NewContext(ctx, logger)

	if pc.DeletionTimestamp != nil {
		ctxInfo(ctx, "ProviderConfig is being deleted, stopping controllers", "providerConfig", pc, "syncId", syncID)

		// Important: We assume that the tenancy service will delete all resources associated with this
		// ProviderConfig before the ProviderConfig itself is deleted. Thus, it is safe to just stop
		// the controllers without performing any resource cleanup here.
		err := c.manager.StopControllersForProviderConfig(ctx, pc)
		if err != nil {
			return fmt.Errorf("failed to stop controllers for providerConfig %v: %w", pc, err)
		}
		return nil
	}

	ctxInfo(ctx, "Syncing providerConfig", "providerConfig", pc, "syncId", syncID)
	err = c.manager.StartControllersForProviderConfig(ctx, pc)
	if err != nil {
		return fmt.Errorf("failed to start controllers for providerConfig %v: %w", pc, err)
	}

	ctxInfo(ctx, "Successfully synced providerConfig", "providerConfig", pc, "syncId", syncID)
	return nil
}

func ctxInfo(ctx context.Context, msg string, keysAndValues ...interface{}) {
	klog.FromContext(ctx).Info(msg, keysAndValues...)
}

func ctxErrorf(ctx context.Context, format string, args ...interface{}) {
	klog.FromContext(ctx).Error(nil, fmt.Sprintf(format, args...))
}

func ctxInfoDepth(ctx context.Context, depth int, msg string, keysAndValues ...interface{}) {
	klog.FromContext(ctx).Info(msg, keysAndValues...)
}
