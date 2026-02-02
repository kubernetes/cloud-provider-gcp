load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "gcustom",
    srcs = ["make_matcher.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/onsi/gomega/gcustom",
    importpath = "github.com/onsi/gomega/gcustom",
    visibility = ["//visibility:public"],
    deps = ["//vendor/github.com/onsi/gomega/format"],
)
