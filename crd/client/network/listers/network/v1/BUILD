load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "network",
    srcs = [
        "expansion_generated.go",
        "gkenetworkparamset.go",
        "network.go",
        "networkinterface.go",
    ],
    importpath = "k8s.io/cloud-provider-gcp/crd/client/network/listers/network/v1",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/api/errors",
        "//vendor/k8s.io/apimachinery/pkg/labels",
        "//vendor/k8s.io/client-go/tools/cache",
        "//vendor/k8s.io/cloud-provider-gcp/crd/apis/network/v1:network",
    ],
)
