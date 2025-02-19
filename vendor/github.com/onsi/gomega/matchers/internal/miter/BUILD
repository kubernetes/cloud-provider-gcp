load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "miter",
    srcs = [
        "type_support_iter.go",
        "type_support_noiter.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/onsi/gomega/matchers/internal/miter",
    importpath = "github.com/onsi/gomega/matchers/internal/miter",
    visibility = ["//vendor/github.com/onsi/gomega/matchers:__subpackages__"],
)
