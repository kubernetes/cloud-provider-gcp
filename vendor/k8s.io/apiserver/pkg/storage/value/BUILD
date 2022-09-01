load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "value",
    srcs = [
        "metrics.go",
        "transformer.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/storage/value",
    importpath = "k8s.io/apiserver/pkg/storage/value",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/google.golang.org/grpc/status",
        "//vendor/k8s.io/apimachinery/pkg/util/errors",
        "//vendor/k8s.io/component-base/metrics",
        "//vendor/k8s.io/component-base/metrics/legacyregistry",
    ],
)
