load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "retry",
    srcs = ["util.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/client-go/util/retry",
    importpath = "k8s.io/client-go/util/retry",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/api/errors",
        "//vendor/k8s.io/apimachinery/pkg/util/wait",
    ],
)
