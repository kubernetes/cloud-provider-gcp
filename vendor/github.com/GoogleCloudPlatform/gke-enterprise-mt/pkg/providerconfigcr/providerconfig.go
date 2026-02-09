// Package providerconfig processes a ProviderConfig CR into a more consumable golang struct
package providerconfig

import (
	"context"
	"fmt"

	providerconfigv1 "github.com/GoogleCloudPlatform/gke-enterprise-mt/apis/providerconfig/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

var (
	// ProviderConfigGVK is the GroupVersionKind for the ProviderConfig CR.
	ProviderConfigGVK = schema.GroupVersionKind{
		Group:   providerconfigv1.GroupVersion.Group,
		Version: providerconfigv1.GroupVersion.Version,
		Kind:    "ProviderConfig",
	}
	// ProviderConfigGVR is the GroupVersionResource for the ProviderConfig CR.
	ProviderConfigGVR = schema.GroupVersionResource{
		Group:    providerconfigv1.GroupVersion.Group,
		Version:  providerconfigv1.GroupVersion.Version,
		Resource: "providerconfigs",
	}
)

// NewProviderConfig creates a new ProviderConfig from the given k8s object object.
func NewProviderConfig(obj any) (*providerconfigv1.ProviderConfig, error) {
	unstructuredObj, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return nil, fmt.Errorf("object of type %T could not be cast to *unstructured.Unstructured", obj)
	}

	if unstructuredObj.GroupVersionKind() != ProviderConfigGVK {
		return nil, fmt.Errorf("unstructured object is not a ProviderConfig, got %v", unstructuredObj.GroupVersionKind())
	}

	pc := &providerconfigv1.ProviderConfig{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.Object, pc); err != nil {
		return nil, err
	}
	return pc, nil
}

// NewProviderConfigFromClient fetches a ProviderConfig from the client and returns it as a struct.
func NewProviderConfigFromClient(ctx context.Context, client dynamic.Interface, name string) (*providerconfigv1.ProviderConfig, error) {
	u, err := client.Resource(ProviderConfigGVR).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return NewProviderConfig(u)
}
