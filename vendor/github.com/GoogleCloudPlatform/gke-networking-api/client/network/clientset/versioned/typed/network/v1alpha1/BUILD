load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "v1alpha1",
    srcs = [
        "doc.go",
        "generated_expansion.go",
        "gkenetworkparamset.go",
        "gkenetworkparamsetlist.go",
        "network.go",
        "network_client.go",
        "networkinterface.go",
        "networkinterfacelist.go",
        "networklist.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/GoogleCloudPlatform/gke-networking-api/client/network/clientset/versioned/typed/network/v1alpha1",
    importpath = "github.com/GoogleCloudPlatform/gke-networking-api/client/network/clientset/versioned/typed/network/v1alpha1",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/GoogleCloudPlatform/gke-networking-api/apis/network/v1alpha1",
        "//vendor/github.com/GoogleCloudPlatform/gke-networking-api/client/network/clientset/versioned/scheme",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/types",
        "//vendor/k8s.io/apimachinery/pkg/watch",
        "//vendor/k8s.io/client-go/gentype",
        "//vendor/k8s.io/client-go/rest",
    ],
)
