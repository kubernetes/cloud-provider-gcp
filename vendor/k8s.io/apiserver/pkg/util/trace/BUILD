load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["trace.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/apiserver/pkg/util/trace",
    importpath = "k8s.io/apiserver/pkg/util/trace",
    visibility = ["//visibility:public"],
    deps = ["//vendor/github.com/golang/glog:go_default_library"],
)
