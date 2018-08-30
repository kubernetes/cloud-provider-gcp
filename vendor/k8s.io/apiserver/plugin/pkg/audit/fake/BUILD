load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "doc.go",
        "fake.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/apiserver/plugin/pkg/audit/fake",
    importpath = "k8s.io/apiserver/plugin/pkg/audit/fake",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apiserver/pkg/apis/audit:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/audit:go_default_library",
    ],
)
