load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["yandex.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/golang.org/x/oauth2/yandex",
    importpath = "golang.org/x/oauth2/yandex",
    visibility = ["//visibility:public"],
    deps = ["//vendor/golang.org/x/oauth2:go_default_library"],
)
