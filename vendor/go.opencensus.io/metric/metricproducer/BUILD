load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "metricproducer",
    srcs = [
        "manager.go",
        "producer.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opencensus.io/metric/metricproducer",
    importpath = "go.opencensus.io/metric/metricproducer",
    visibility = ["//visibility:public"],
    deps = ["//vendor/go.opencensus.io/metric/metricdata"],
)
