load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["httplex.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/golang.org/x/net/lex/httplex",
    importpath = "golang.org/x/net/lex/httplex",
    visibility = ["//visibility:public"],
    deps = ["//vendor/golang.org/x/net/idna:go_default_library"],
)
