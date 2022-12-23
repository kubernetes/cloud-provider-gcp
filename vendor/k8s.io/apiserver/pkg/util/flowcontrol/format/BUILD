load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "format",
    srcs = ["formatting.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/util/flowcontrol/format",
    importpath = "k8s.io/apiserver/pkg/util/flowcontrol/format",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/flowcontrol/v1beta3",
        "//vendor/k8s.io/apiserver/pkg/authentication/user",
        "//vendor/k8s.io/apiserver/pkg/endpoints/request",
    ],
)
