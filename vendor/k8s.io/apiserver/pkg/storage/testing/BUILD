load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "doc.go",
        "types.go",
        "utils.go",
        "zz_generated.deepcopy.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/apiserver/pkg/storage/testing",
    importpath = "k8s.io/apiserver/pkg/storage/testing",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/api/meta:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/runtime:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/storage:go_default_library",
    ],
)
