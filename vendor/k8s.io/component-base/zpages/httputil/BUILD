load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "httputil",
    srcs = ["httputil.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/component-base/zpages/httputil",
    importpath = "k8s.io/component-base/zpages/httputil",
    visibility = ["//visibility:public"],
    deps = ["//vendor/github.com/munnerz/goautoneg"],
)
