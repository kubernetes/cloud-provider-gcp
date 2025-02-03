load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "csrmetrics",
    srcs = ["csrmetrics.go"],
    importpath = "k8s.io/cloud-provider-gcp/pkg/csrmetrics",
    visibility = [
        "//pkg/csrapproval:__subpackages__",
    ],
    deps = [
        "//vendor/github.com/prometheus/client_golang/prometheus",
    ],
)
