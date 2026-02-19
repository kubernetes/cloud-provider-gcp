package framework

import (
	"context"
	"fmt"

	providerconfigv1 "github.com/GoogleCloudPlatform/gke-enterprise-mt/apis/providerconfig/v1"
	crv1 "github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/providerconfigcr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"
)

// manager coordinates lifecycle of controllers scoped to individual ProviderConfigs.
// It ensures per-ProviderConfig controller startup is idempotent, adds/removes
// finalizers, and wires stop channels for clean shutdown.
//
// This manager assumes it is invoked by a workqueue that guarantees
// the same ProviderConfig key is never processed concurrently.
type manager struct {
	controllers *ControllerMap

	client            dynamic.Interface
	finalizerName     string
	controllerStarter ControllerStarter
}

// newManager constructs a new generic ProviderConfig controller manager.
// It does not start any controllers until StartControllersForProviderConfig is invoked.
func newManager(client dynamic.Interface, finalizerName string, controllerStarter ControllerStarter,
) *manager {
	return &manager{
		controllers:       NewControllerMap(),
		client:            client,
		finalizerName:     finalizerName,
		controllerStarter: controllerStarter,
	}
}

// providerConfigKey returns the key for a ProviderConfig in the controller map.
func providerConfigKey(pc *providerconfigv1.ProviderConfig) string {
	return pc.Name
}

func (m *manager) getProviderConfig(ctx context.Context, name string) (*providerconfigv1.ProviderConfig, error) {
	u, err := m.client.Resource(crv1.ProviderConfigGVR).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return crv1.NewProviderConfig(u)
}

// rollbackFinalizerOnStartFailure removes the finalizer after a start failure
// so that ProviderConfig deletion is not blocked.
func (m *manager) rollbackFinalizerOnStartFailure(ctx context.Context, pc *providerconfigv1.ProviderConfig, cause error) {
	pcLatest, err := m.getProviderConfig(ctx, pc.Name)
	if err != nil {
		klog.Errorf("failed to get latest ProviderConfig for finalizer rollback: %v, originalError: %v", err, cause)
		return
	}
	if err := crv1.DeleteFinalizer(ctx, pcLatest, m.client, m.finalizerName); err != nil {
		klog.Errorf("failed to clean up finalizer after start failure: %v, originalError: %v", err, cause)
	}
}

// StartControllersForProviderConfig ensures finalizers are present and starts
// the controllers associated with the given ProviderConfig. The call is
// idempotent: repeated calls for the same ProviderConfig will only start
// controllers once.
func (m *manager) StartControllersForProviderConfig(ctx context.Context, pc *providerconfigv1.ProviderConfig) error {
	if pc.DeletionTimestamp != nil {
		klog.Info("ProviderConfig is terminating; skipping start")
		return nil
	}

	pcKey := providerConfigKey(pc)

	cs, existed := m.controllers.GetOrCreate(pcKey)
	if cs.stopCh != nil {
		klog.Info("Controllers for provider config already exist, skipping start")
		return nil
	}

	klog.Info("Starting controllers for provider config")

	hadFinalizer := false
	for _, finalizer := range pc.Finalizers {
		if finalizer == m.finalizerName {
			hadFinalizer = true
			break
		}
	}

	if !hadFinalizer {
		if err := crv1.EnsureFinalizer(ctx, pc, m.client, m.finalizerName); err != nil {
			if !existed {
				m.controllers.Delete(pcKey)
			}
			return fmt.Errorf("failed to ensure finalizer %s for provider config %s: %w", m.finalizerName, pcKey, err)
		}
	}

	controllerStopCh, err := m.controllerStarter.StartController(pc)
	if err == nil && controllerStopCh == nil {
		err = fmt.Errorf("controller starter returned nil channel")
	}
	if err != nil {
		if !existed {
			m.controllers.Delete(pcKey)
		}
		if !hadFinalizer {
			m.rollbackFinalizerOnStartFailure(ctx, pc, err)
		}
		return fmt.Errorf("failed to start controller for provider config %s: %w", pcKey, err)
	}

	cs.stopCh = controllerStopCh

	klog.Info("Started controllers for provider config")
	return nil
}

// StopControllersForProviderConfig stops the controllers for the given ProviderConfig
// and removes the associated finalizer. Finalizer removal is attempted even if no
// controller mapping exists, ensuring deletion can proceed after process restarts
// or when controllers were previously stopped.
func (m *manager) StopControllersForProviderConfig(ctx context.Context, pc *providerconfigv1.ProviderConfig) error {
	pcKey := providerConfigKey(pc)

	if cs, exists := m.controllers.Get(pcKey); exists {
		m.controllers.Delete(pcKey)
		if cs.stopCh != nil {
			close(cs.stopCh)
			klog.Info("Signaled controller stop")
		} else {
			klog.Info("Controllers for provider config already stopped")
		}
	} else {
		klog.Info("Controllers for provider config do not exist")
	}

	// Fetch the latest ProviderConfig to ensure we have current finalizer state.
	latestPC, err := m.getProviderConfig(ctx, pc.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.Info("ProviderConfig not found while stopping controllers; skipping finalizer removal")
			return nil
		}
		return fmt.Errorf("Failed to get latest ProviderConfig for finalizer removal: %w", err)
	}

	if err := crv1.DeleteFinalizer(ctx, latestPC, m.client, m.finalizerName); err != nil {
		return fmt.Errorf("Failed to delete finalizer %s for provider config %s: %w", m.finalizerName, pcKey, err)
	}
	klog.Info("Stopped controllers for provider config")
	return nil
}
