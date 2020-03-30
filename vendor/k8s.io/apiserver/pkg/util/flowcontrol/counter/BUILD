load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "interface.go",
        "noop.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/util/flowcontrol/counter",
    importpath = "k8s.io/apiserver/pkg/util/flowcontrol/counter",
    visibility = ["//visibility:public"],
)
