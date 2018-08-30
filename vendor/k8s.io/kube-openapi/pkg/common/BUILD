load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "common.go",
        "doc.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kube-openapi/pkg/common",
    importpath = "k8s.io/kube-openapi/pkg/common",
    visibility = ["//visibility:public"],
    deps = [
        "@com_github_emicklei_go_restful//:go_default_library",
        "@com_github_go_openapi_spec//:go_default_library",
    ],
)
