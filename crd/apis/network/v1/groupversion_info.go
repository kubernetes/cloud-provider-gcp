// Package v1 contains API Schema definitions for the networking v1 API group
// +kubebuilder:object:generate=true
// +groupName=networking.gke.io

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// GroupVersion is group version used to register these objects
	GroupVersion = schema.GroupVersion{Group: "networking.gke.io", Version: "v1"}

	schemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	AddToScheme   = schemeBuilder.AddToScheme
)

// Kind names for the Network objects.
const (
	KindNetwork          = "Network"
	KindNetworkInterface = "NetworkInterface"
)

// Adds the list of known types to Scheme.
func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(GroupVersion,
		&Network{},
		&NetworkList{},
		&NetworkInterface{},
		&NetworkInterfaceList{},
	)
	metav1.AddToGroupVersion(scheme, GroupVersion)
	return nil
}

// Kind takes an unqualified kind and returns back a Group qualified GroupKind
func Kind(kind string) schema.GroupKind {
	return GroupVersion.WithKind(kind).GroupKind()
}
