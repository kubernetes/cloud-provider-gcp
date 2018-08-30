load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["writer.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/github.com/googleapis/gnostic/jsonwriter",
    importpath = "github.com/googleapis/gnostic/jsonwriter",
    visibility = ["//visibility:public"],
    deps = ["//vendor/gopkg.in/yaml.v2:go_default_library"],
)
