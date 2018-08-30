load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["fuzzer.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/apiserver/pkg/apis/audit/fuzzer",
    importpath = "k8s.io/apiserver/pkg/apis/audit/fuzzer",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/google/gofuzz:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/runtime:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/runtime/serializer:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/apis/audit:go_default_library",
    ],
)
