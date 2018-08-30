load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["container.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/cloud.google.com/go/container",
    importpath = "cloud.google.com/go/container",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/golang.org/x/net/context:go_default_library",
        "//vendor/google.golang.org/api/container/v1:go_default_library",
        "//vendor/google.golang.org/api/option:go_default_library",
        "//vendor/google.golang.org/api/transport/http:go_default_library",
    ],
)
