load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "ratelimiter",
    srcs = ["rate_limiter.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/component-base/metrics/prometheus/ratelimiter",
    importpath = "k8s.io/component-base/metrics/prometheus/ratelimiter",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/client-go/util/flowcontrol",
        "//vendor/k8s.io/component-base/metrics",
        "//vendor/k8s.io/component-base/metrics/legacyregistry",
    ],
)
