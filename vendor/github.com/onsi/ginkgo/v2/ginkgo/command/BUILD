load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "command",
    srcs = [
        "abort.go",
        "command.go",
        "program.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/onsi/ginkgo/v2/ginkgo/command",
    importpath = "github.com/onsi/ginkgo/v2/ginkgo/command",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/onsi/ginkgo/v2/formatter",
        "//vendor/github.com/onsi/ginkgo/v2/types",
    ],
)
