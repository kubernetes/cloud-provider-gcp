load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["writer.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/github.com/mailru/easyjson/jwriter",
    importpath = "github.com/mailru/easyjson/jwriter",
    visibility = ["//visibility:public"],
    deps = ["//vendor/github.com/mailru/easyjson/buffer:go_default_library"],
)
