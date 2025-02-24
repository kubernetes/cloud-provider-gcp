load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "autoscaling",
    srcs = [
        "expansion_generated.go",
        "horizontalpodautoscaler.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/client-go/listers/autoscaling/v2",
    importpath = "k8s.io/client-go/listers/autoscaling/v2",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/autoscaling/v2:autoscaling",
        "//vendor/k8s.io/apimachinery/pkg/labels",
        "//vendor/k8s.io/client-go/listers",
        "//vendor/k8s.io/client-go/tools/cache",
    ],
)
