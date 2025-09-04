load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "util",
    srcs = ["util.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/client-go/tools/record/util",
    importpath = "k8s.io/client-go/tools/record/util",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/google/uuid",
        "//vendor/k8s.io/api/core/v1:core",
        "//vendor/k8s.io/apimachinery/pkg/api/errors",
        "//vendor/k8s.io/apimachinery/pkg/api/validation",
    ],
)
