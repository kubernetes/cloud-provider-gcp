load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "promlint",
    srcs = ["promlint.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/prometheus/client_golang/prometheus/testutil/promlint",
    importpath = "github.com/prometheus/client_golang/prometheus/testutil/promlint",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/prometheus/client_model/go",
        "//vendor/github.com/prometheus/common/expfmt",
    ],
)
