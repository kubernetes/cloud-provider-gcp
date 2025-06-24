load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "network",
    srcs = [
        "gkenetworkparamset.go",
        "interface.go",
        "network.go",
        "networkinterface.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/GoogleCloudPlatform/gke-networking-api/client/network/informers/externalversions/network/v1",
    importpath = "github.com/GoogleCloudPlatform/gke-networking-api/client/network/informers/externalversions/network/v1",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/GoogleCloudPlatform/gke-networking-api/apis/network/v1:network",
        "//vendor/github.com/GoogleCloudPlatform/gke-networking-api/client/network/clientset/versioned",
        "//vendor/github.com/GoogleCloudPlatform/gke-networking-api/client/network/informers/externalversions/internalinterfaces",
        "//vendor/github.com/GoogleCloudPlatform/gke-networking-api/client/network/listers/network/v1:network",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/watch",
        "//vendor/k8s.io/client-go/tools/cache",
    ],
)
