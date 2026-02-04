load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "format",
    srcs = ["format.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/kubernetes/test/utils/format",
    importpath = "k8s.io/kubernetes/test/utils/format",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/onsi/gomega/format",
        "//vendor/sigs.k8s.io/yaml",
    ],
)
