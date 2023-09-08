load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "options",
    srcs = ["nodeipamcontroller.go"],
    importpath = "k8s.io/cloud-provider-gcp/cloud-controller-manager/cmd/cloud-controller-manager/options",
    visibility = ["//visibility:public"],
    deps = [
        "//cloud-controller-manager/pkg/controller/nodeipam/config",
        "//vendor/github.com/spf13/pflag",
    ],
)
