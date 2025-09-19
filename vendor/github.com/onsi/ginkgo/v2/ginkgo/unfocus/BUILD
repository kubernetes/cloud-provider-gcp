load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "unfocus",
    srcs = ["unfocus_command.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/onsi/ginkgo/v2/ginkgo/unfocus",
    importpath = "github.com/onsi/ginkgo/v2/ginkgo/unfocus",
    visibility = ["//visibility:public"],
    deps = ["//vendor/github.com/onsi/ginkgo/v2/ginkgo/command"],
)
