load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "filter",
    srcs = ["filter.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/filter",
    importpath = "github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/filter",
    visibility = ["//visibility:public"],
    deps = ["//vendor/k8s.io/klog/v2:klog"],
)
