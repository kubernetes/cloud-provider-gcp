load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "gutil",
    srcs = [
        "post_io.go",
        "using_io.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/onsi/gomega/internal/gutil",
    importpath = "github.com/onsi/gomega/internal/gutil",
    visibility = ["//vendor/github.com/onsi/gomega:__subpackages__"],
)
