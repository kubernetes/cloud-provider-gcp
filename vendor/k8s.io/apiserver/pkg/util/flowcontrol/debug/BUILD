load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "debug",
    srcs = ["dump.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/util/flowcontrol/debug",
    importpath = "k8s.io/apiserver/pkg/util/flowcontrol/debug",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apiserver/pkg/endpoints/request",
        "//vendor/k8s.io/apiserver/pkg/util/flowcontrol/request",
    ],
)
