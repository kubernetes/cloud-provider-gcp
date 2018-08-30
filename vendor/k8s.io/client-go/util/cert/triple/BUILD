load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["triple.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/client-go/util/cert/triple",
    importpath = "k8s.io/client-go/util/cert/triple",
    visibility = ["//visibility:public"],
    deps = ["//vendor/k8s.io/client-go/util/cert:go_default_library"],
)
