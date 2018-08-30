load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "extension.go",
        "openapi.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kube-openapi/pkg/generators",
    importpath = "k8s.io/kube-openapi/pkg/generators",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/golang/glog:go_default_library",
        "//vendor/k8s.io/kube-openapi/pkg/common:go_default_library",
        "@io_k8s_gengo//args:go_default_library",
        "@io_k8s_gengo//examples/set-gen/sets:go_default_library",
        "@io_k8s_gengo//generator:go_default_library",
        "@io_k8s_gengo//namer:go_default_library",
        "@io_k8s_gengo//types:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = [
        "extension_test.go",
        "openapi_test.go",
    ],
    embed = [":go_default_library"],
    deps = [
        "@com_github_stretchr_testify//assert:go_default_library",
        "@io_k8s_gengo//examples/set-gen/sets:go_default_library",
        "@io_k8s_gengo//generator:go_default_library",
        "@io_k8s_gengo//namer:go_default_library",
        "@io_k8s_gengo//parser:go_default_library",
        "@io_k8s_gengo//types:go_default_library",
    ],
)
