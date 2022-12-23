load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "cached",
    srcs = [
        "client.go",
        "groupversion.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/client-go/openapi/cached",
    importpath = "k8s.io/client-go/openapi/cached",
    visibility = ["//visibility:public"],
    deps = ["//vendor/k8s.io/client-go/openapi"],
)
