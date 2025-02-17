load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "fake",
    srcs = [
        "clientset_generated.go",
        "doc.go",
        "register.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/GoogleCloudPlatform/gke-networking-api/client/network/clientset/versioned/fake",
    importpath = "github.com/GoogleCloudPlatform/gke-networking-api/client/network/clientset/versioned/fake",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/GoogleCloudPlatform/gke-networking-api/apis/network/v1:network",
        "//vendor/github.com/GoogleCloudPlatform/gke-networking-api/apis/network/v1alpha1",
        "//vendor/github.com/GoogleCloudPlatform/gke-networking-api/client/network/clientset/versioned",
        "//vendor/github.com/GoogleCloudPlatform/gke-networking-api/client/network/clientset/versioned/typed/network/v1:network",
        "//vendor/github.com/GoogleCloudPlatform/gke-networking-api/client/network/clientset/versioned/typed/network/v1/fake",
        "//vendor/github.com/GoogleCloudPlatform/gke-networking-api/client/network/clientset/versioned/typed/network/v1alpha1",
        "//vendor/github.com/GoogleCloudPlatform/gke-networking-api/client/network/clientset/versioned/typed/network/v1alpha1/fake",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
        "//vendor/k8s.io/apimachinery/pkg/runtime/serializer",
        "//vendor/k8s.io/apimachinery/pkg/util/runtime",
        "//vendor/k8s.io/apimachinery/pkg/watch",
        "//vendor/k8s.io/client-go/discovery",
        "//vendor/k8s.io/client-go/discovery/fake",
        "//vendor/k8s.io/client-go/testing",
    ],
)
