load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "doc.go",
        "openapi.go",
        "util.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kube-openapi/pkg/builder",
    importpath = "k8s.io/kube-openapi/pkg/builder",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/kube-openapi/pkg/common:go_default_library",
        "//vendor/k8s.io/kube-openapi/pkg/util:go_default_library",
        "@com_github_emicklei_go_restful//:go_default_library",
        "@com_github_go_openapi_spec//:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = ["openapi_test.go"],
    embed = [":go_default_library"],
    deps = [
        "//vendor/k8s.io/kube-openapi/pkg/common:go_default_library",
        "@com_github_emicklei_go_restful//:go_default_library",
        "@com_github_go_openapi_spec//:go_default_library",
        "@com_github_stretchr_testify//assert:go_default_library",
    ],
)
