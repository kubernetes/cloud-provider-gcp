load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "network",
    srcs = [
        "annotations.go",
        "doc.go",
        "network.go",
        "network_types.go",
        "networkinterface_types.go",
        "zz_generated.deepcopy.go",
        "zz_generated.register.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/cloud-provider-gcp/crd/apis/network/v1",
    importpath = "k8s.io/cloud-provider-gcp/crd/apis/network/v1",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
    ],
)
