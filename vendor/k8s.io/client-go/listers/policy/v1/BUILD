load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "policy",
    srcs = [
        "eviction.go",
        "expansion_generated.go",
        "poddisruptionbudget.go",
        "poddisruptionbudget_expansion.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/client-go/listers/policy/v1",
    importpath = "k8s.io/client-go/listers/policy/v1",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/core/v1:core",
        "//vendor/k8s.io/api/policy/v1:policy",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/labels",
        "//vendor/k8s.io/client-go/listers",
        "//vendor/k8s.io/client-go/tools/cache",
    ],
)
