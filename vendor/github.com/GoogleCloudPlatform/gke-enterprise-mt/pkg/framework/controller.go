// Package framework provides a generic controller implementation for managing
// the lifecycle of controllers that are scoped to ProviderConfig resources.
//
// It handles watching ProviderConfig resources, ensuring finalizers are present,
// and starting/stopping the associated controllers using a provided
// ControllerStarter implementation.
package framework

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"runtime/debug"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"

	"k8s.io/klog/v2"
	mtcontext "github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/framework/mtcontext"
	"github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/framework/taskqueue"
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
	StartController(pc *unstructured.Unstructured) (chan<- struct{}, error)
}

const (
	providerConfigControllerName = "provider-config-controller"
	resourceName                 = "provider-configs"
	workersCount                 = 5
)

// controllerManager implements the logic for starting and stopping controllers for each ProviderConfig.
type controllerManager interface {
	StartControllersForProviderConfig(ctx context.Context, pc *unstructured.Unstructured) error
	StopControllersForProviderConfig(ctx context.Context, pc *unstructured.Unstructured) error
	ForceCleanupTenant(pcKey string)
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
				klog.V(4).InfoS("Enqueue add event", "object", obj)
				c.providerConfigQueue.Enqueue(obj)
			},
			UpdateFunc: func(old, cur any) {
				klog.V(4).InfoS("Enqueue update event", "old", old, "new", cur)
				c.providerConfigQueue.Enqueue(cur)
			},
			DeleteFunc: func(obj any) {
				klog.V(4).InfoS("Enqueue delete event", "object", obj)
				c.providerConfigQueue.Enqueue(obj)
			},
		})

	klog.InfoS("ProviderConfig controller created")
	return c
}

// Run starts the controller and blocks until the stop channel is closed.
func (c *Controller) Run() {
	defer c.shutdown()

	klog.InfoS("Starting ProviderConfig controller")

	klog.InfoS("Waiting for initial cache sync before starting ProviderConfig Controller")
	ok := cache.WaitForCacheSync(c.stopCh, c.hasSynced)
	if !ok {
		klog.Error("Failed to wait for initial cache sync before starting ProviderConfig Controller")
		return
	}

	klog.InfoS("Started ProviderConfig Controller", "numWorkers", c.workersCount)
	c.providerConfigQueue.Run()

	<-c.stopCh
	klog.InfoS("ProviderConfig Controller exited")
}

func (c *Controller) shutdown() {
	klog.InfoS("Shutting down ProviderConfig Controller")
	c.providerConfigQueue.Shutdown()
}

func (c *Controller) syncWrapper(ctx context.Context, key string) (err error) {
	// For cluster-scoped resources, the key is the resource name, which is the tenant UID.
	tenantUID := key
	syncID := rand.Int31()

	defer func() {
		if r := recover(); r != nil {
			stack := string(debug.Stack())
			klog.ErrorS(errors.New("panic in ProviderConfig sync worker goroutine"), "Recovered from panic", "panic", r, "stack", stack, "syncID", syncID, "tenant", tenantUID)
			err = fmt.Errorf("panic in sync worker: %v", r)
		}
	}()

	err = c.sync(ctx, key, syncID)
	if err != nil {
		klog.ErrorS(err, "Error syncing providerConfig", "key", key, "syncID", syncID, "tenant", tenantUID)
	}
	return err
}

func (c *Controller) sync(ctx context.Context, key string, syncID int32) (err error) {
	// Tenant UID is not available from context yet. The key itself is the tenant UID.
	tenantUID := key

	obj, exists, err := c.providerConfigLister.GetByKey(key)
	if err != nil {
		return fmt.Errorf("failed to lookup providerConfig for key %s: %w", key, err)
	}
	if !exists || obj == nil {
		klog.InfoS("ProviderConfig does not exist anymore", "key", key, "syncID", syncID, "tenant", tenantUID)
		c.manager.ForceCleanupTenant(key)
		return nil
	}

	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return fmt.Errorf("expected *unstructured.Unstructured but got %T", obj)
	}

	// The tenant UID is the name of the ProviderConfig.
	if tenantUID != u.GetName() {
		err := fmt.Errorf("mismatched tenant UID: %s != %s", tenantUID, u.GetName())
		klog.ErrorS(err, "Mismatched tenant UID", "key", key, "pcName", u.GetName(), "syncID", syncID)
		return err
	}

	// Populate tenant context
	ctx = mtcontext.ContextWithTenantUID(ctx, u.GetName())

	if !u.GetDeletionTimestamp().IsZero() {
		klog.InfoS("ProviderConfig is being deleted, stopping controllers", "providerConfig", u, "syncID", syncID, "tenant", tenantUID)

		err := c.manager.StopControllersForProviderConfig(ctx, u)
		if err != nil {
			return fmt.Errorf("failed to stop controllers for providerConfig %s: %w", u.GetName(), err)
		}
		return nil
	}

	klog.InfoS("Syncing providerConfig", "providerConfig", u, "syncID", syncID, "tenant", tenantUID)
	err = c.manager.StartControllersForProviderConfig(ctx, u)
	if err != nil {
		return fmt.Errorf("failed to start controllers for providerConfig %s: %w", u.GetName(), err)
	}

	klog.InfoS("Successfully synced providerConfig", "providerConfig", u, "syncID", syncID, "tenant", tenantUID)
	return nil

}
