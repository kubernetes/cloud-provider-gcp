package providerconfig

import (
	"context"
	"fmt"
	"slices"

	pcv1 "github.com/GoogleCloudPlatform/gke-enterprise-mt/apis/providerconfig/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
)

// EnsureFinalizer adds the given finalizer to ProviderConfig if it is not
// already present. It uses a retry loop with optimistic locking to ensure
// atomicity and avoid duplicates.
func EnsureFinalizer(ctx context.Context, pc *pcv1.ProviderConfig, dynamicClient dynamic.Interface, finalizerName string) error {
	klog.Infof("Adding finalizer %s to ProviderConfig %s", finalizerName, pc.Name)
	return AddFinalizer(ctx, pc, dynamicClient, finalizerName)
}

// DeleteFinalizer removes the given finalizer from ProviderConfig if it is
// present. It uses a retry loop with optimistic locking to ensure safety.
func DeleteFinalizer(ctx context.Context, pc *pcv1.ProviderConfig, dynamicClient dynamic.Interface, finalizerName string) error {
	klog.Infof("Removing finalizer %s from ProviderConfig %s", finalizerName, pc.Name)
	return RemoveFinalizer(ctx, pc, dynamicClient, finalizerName)
}

// AddFinalizer adds the finalizer to the ProviderConfig using a standard Update loop.
func AddFinalizer(ctx context.Context, pc *pcv1.ProviderConfig, dynamicClient dynamic.Interface, finalizerName string) error {
	client := dynamicClient.Resource(ProviderConfigGVR).Namespace(pc.Namespace)

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		obj, err := client.Get(ctx, pc.Name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get ProviderConfig %s: %w", pc.Name, err)
		}

		finalizers := obj.GetFinalizers()
		if slices.Contains(finalizers, finalizerName) {
			// Already exists, nothing to do.
			return nil
		}

		obj.SetFinalizers(append(finalizers, finalizerName))
		_, err = client.Update(ctx, obj, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update ProviderConfig %s: %w", pc.Name, err)
		}
		return nil
	})
	if err == nil {
		return ctx.Err()
	}
	return err
}

// RemoveFinalizer removes the finalizer from the ProviderConfig using a standard Update loop.
func RemoveFinalizer(ctx context.Context, pc *pcv1.ProviderConfig, dynamicClient dynamic.Interface, finalizerName string) error {
	client := dynamicClient.Resource(ProviderConfigGVR).Namespace(pc.Namespace)

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		obj, err := client.Get(ctx, pc.Name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get ProviderConfig %s: %w", pc.Name, err)
		}

		finalizers := obj.GetFinalizers()
		idx := slices.Index(finalizers, finalizerName)
		if idx == -1 {
			return nil
		}
		obj.SetFinalizers(slices.Delete(finalizers, idx, idx+1))

		_, err = client.Update(ctx, obj, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update ProviderConfig %s: %w", pc.Name, err)
		}
		return nil
	})
	if err == nil {
		return ctx.Err()
	}
	return err
}
