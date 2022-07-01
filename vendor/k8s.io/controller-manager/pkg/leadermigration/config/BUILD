load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "config.go",
        "default.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/controller-manager/pkg/leadermigration/config",
    importpath = "k8s.io/controller-manager/pkg/leadermigration/config",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/runtime:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/runtime/serializer:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/runtime:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/validation/field:go_default_library",
        "//vendor/k8s.io/controller-manager/config:go_default_library",
        "//vendor/k8s.io/controller-manager/config/v1:go_default_library",
        "//vendor/k8s.io/controller-manager/config/v1alpha1:go_default_library",
        "//vendor/k8s.io/controller-manager/config/v1beta1:go_default_library",
    ],
)
