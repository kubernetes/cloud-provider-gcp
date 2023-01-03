package v1

import "k8s.io/apimachinery/pkg/runtime/schema"

// Kind names for the Network objects.
const (
	KindNetwork          = "Network"
	KindNetworkInterface = "NetworkInterface"
)

// Kind takes an unqualified kind and returns back a Group qualified GroupKind
func Kind(kind string) schema.GroupKind {
	return GroupVersion.WithKind(kind).GroupKind()
}
