load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["nodeipamcontroller.go"],
    importpath = "k8s.io/cloud-provider-gcp/cmd/cloud-controller-manager/options",
    visibility = ["//visibility:public"],
    deps = [
        "//pkg/controller/nodeipam/config:go_default_library",
        "//vendor/github.com/spf13/pflag:go_default_library",
    ],
)
