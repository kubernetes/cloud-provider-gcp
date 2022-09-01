load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "testutil",
    srcs = [
        "metrics.go",
        "promlint.go",
        "testutil.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/component-base/metrics/testutil",
    importpath = "k8s.io/component-base/metrics/testutil",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/prometheus/client_golang/prometheus/testutil",
        "//vendor/github.com/prometheus/client_golang/prometheus/testutil/promlint",
        "//vendor/github.com/prometheus/client_model/go",
        "//vendor/github.com/prometheus/common/expfmt",
        "//vendor/github.com/prometheus/common/model",
        "//vendor/k8s.io/apimachinery/pkg/version",
        "//vendor/k8s.io/component-base/metrics",
    ],
)
