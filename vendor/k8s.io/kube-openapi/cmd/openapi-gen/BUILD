load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["openapi-gen.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kube-openapi/cmd/openapi-gen",
    importpath = "k8s.io/kube-openapi/cmd/openapi-gen",
    visibility = ["//visibility:private"],
    deps = [
        "//vendor/k8s.io/kube-openapi/pkg/generators:go_default_library",
        "@io_k8s_gengo//args:go_default_library",
    ],
)

go_binary(
    name = "openapi-gen",
    embed = [":go_default_library"],
    visibility = ["//visibility:public"],
)
