package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
    "go_test",
)

go_test(
    name = "go_default_test",
    srcs = ["scheme_test.go"],
    embed = [":go_default_library"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/api/testing/roundtrip:go_default_library",
        "@io_k8s_sample_apiserver//pkg/apis/wardle/fuzzer:go_default_library",
    ],
)

go_library(
    name = "go_default_library",
    srcs = ["apiserver.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/staging/src/k8s.io/sample-apiserver/pkg/apiserver",
    importpath = "k8s.io/kubernetes/staging/src/k8s.io/sample-apiserver/pkg/apiserver",
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/runtime:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/runtime/serializer:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/version:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/registry/rest:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/server:go_default_library",
        "@io_k8s_sample_apiserver//pkg/apis/wardle:go_default_library",
        "@io_k8s_sample_apiserver//pkg/apis/wardle/install:go_default_library",
        "@io_k8s_sample_apiserver//pkg/registry:go_default_library",
        "@io_k8s_sample_apiserver//pkg/registry/wardle/fischer:go_default_library",
        "@io_k8s_sample_apiserver//pkg/registry/wardle/flunder:go_default_library",
    ],
)

filegroup(
    name = "package-srcs",
    srcs = glob(["**"]),
    tags = ["automanaged"],
    visibility = ["//visibility:private"],
)

filegroup(
    name = "all-srcs",
    srcs = [":package-srcs"],
    tags = ["automanaged"],
)
