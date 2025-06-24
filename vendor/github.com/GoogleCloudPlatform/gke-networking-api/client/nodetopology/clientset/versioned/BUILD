load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "versioned",
    srcs = ["clientset.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/GoogleCloudPlatform/gke-networking-api/client/nodetopology/clientset/versioned",
    importpath = "github.com/GoogleCloudPlatform/gke-networking-api/client/nodetopology/clientset/versioned",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/GoogleCloudPlatform/gke-networking-api/client/nodetopology/clientset/versioned/typed/nodetopology/v1:nodetopology",
        "//vendor/k8s.io/client-go/discovery",
        "//vendor/k8s.io/client-go/rest",
        "//vendor/k8s.io/client-go/util/flowcontrol",
    ],
)
