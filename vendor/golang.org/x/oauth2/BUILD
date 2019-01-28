load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "oauth2.go",
        "token.go",
        "transport.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/golang.org/x/oauth2",
    importpath = "golang.org/x/oauth2",
    visibility = ["//visibility:public"],
    deps = ["//vendor/golang.org/x/oauth2/internal:go_default_library"],
)
