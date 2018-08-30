load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["main.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kube-openapi/test/integration/builder",
    importpath = "k8s.io/kube-openapi/test/integration/builder",
    visibility = ["//visibility:private"],
    deps = [
        "//vendor/k8s.io/kube-openapi/pkg/builder:go_default_library",
        "//vendor/k8s.io/kube-openapi/pkg/common:go_default_library",
        "//vendor/k8s.io/kube-openapi/test/integration/pkg/generated:go_default_library",
        "@com_github_go_openapi_spec//:go_default_library",
    ],
)

go_binary(
    name = "builder",
    embed = [":go_default_library"],
    visibility = ["//visibility:public"],
)
