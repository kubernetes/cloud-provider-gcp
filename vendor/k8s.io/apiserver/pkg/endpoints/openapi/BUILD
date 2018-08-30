load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["openapi.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/apiserver/pkg/endpoints/openapi",
    importpath = "k8s.io/apiserver/pkg/endpoints/openapi",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/runtime:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema:go_default_library",
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
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/runtime:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/endpoints/openapi/testing:go_default_library",
        "@com_github_go_openapi_spec//:go_default_library",
    ],
)
