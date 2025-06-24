load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "config",
    srcs = [
        "config.go",
        "default.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/controller-manager/pkg/leadermigration/config",
    importpath = "k8s.io/controller-manager/pkg/leadermigration/config",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/runtime/serializer",
        "//vendor/k8s.io/apimachinery/pkg/util/runtime",
        "//vendor/k8s.io/apimachinery/pkg/util/validation/field",
        "//vendor/k8s.io/controller-manager/config",
        "//vendor/k8s.io/controller-manager/config/v1:config",
        "//vendor/k8s.io/controller-manager/config/v1alpha1",
        "//vendor/k8s.io/controller-manager/config/v1beta1",
    ],
)
