load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "node",
    srcs = [
        "expansion_generated.go",
        "runtimeclass.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/client-go/listers/node/v1",
    importpath = "k8s.io/client-go/listers/node/v1",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/node/v1:node",
        "//vendor/k8s.io/apimachinery/pkg/labels",
        "//vendor/k8s.io/client-go/listers",
        "//vendor/k8s.io/client-go/tools/cache",
    ],
)
