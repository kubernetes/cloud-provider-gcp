load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "csrmetrics",
    srcs = ["csrmetrics.go"],
    importpath = "k8s.io/cloud-provider-gcp/pkg/csrmetrics",
    visibility = [
        "//cmd/gcp-controller-manager:__subpackages__",
        "//pkg/csrapproval:__subpackages__",
    ],
    deps = [
        "//vendor/github.com/prometheus/client_golang/prometheus",
    ],
)
