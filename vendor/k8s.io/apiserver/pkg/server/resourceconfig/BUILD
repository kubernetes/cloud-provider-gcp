load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "resourceconfig",
    srcs = [
        "doc.go",
        "helpers.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/server/resourceconfig",
    importpath = "k8s.io/apiserver/pkg/server/resourceconfig",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
        "//vendor/k8s.io/apiserver/pkg/server/storage",
        "//vendor/k8s.io/component-base/cli/flag",
    ],
)
