load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "service",
    srcs = [
        "util.go",
        "warnings.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/kubernetes/pkg/api/service",
    importpath = "k8s.io/kubernetes/pkg/api/service",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/util/validation",
        "//vendor/k8s.io/apimachinery/pkg/util/validation/field",
        "//vendor/k8s.io/kubernetes/pkg/apis/core",
        "//vendor/k8s.io/kubernetes/pkg/apis/core/helper",
        "//vendor/k8s.io/utils/net",
    ],
)
