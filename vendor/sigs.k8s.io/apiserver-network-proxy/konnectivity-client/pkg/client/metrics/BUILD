load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "metrics",
    srcs = ["metrics.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/sigs.k8s.io/apiserver-network-proxy/konnectivity-client/pkg/client/metrics",
    importpath = "sigs.k8s.io/apiserver-network-proxy/konnectivity-client/pkg/client/metrics",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/prometheus/client_golang/prometheus",
        "//vendor/sigs.k8s.io/apiserver-network-proxy/konnectivity-client/pkg/common/metrics",
        "//vendor/sigs.k8s.io/apiserver-network-proxy/konnectivity-client/proto/client",
    ],
)
