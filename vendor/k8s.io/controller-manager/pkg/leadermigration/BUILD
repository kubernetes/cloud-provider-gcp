load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "feature.go",
        "filter.go",
        "migrator.go",
        "util.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/controller-manager/pkg/leadermigration",
    importpath = "k8s.io/controller-manager/pkg/leadermigration",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apiserver/pkg/util/feature:go_default_library",
        "//vendor/k8s.io/controller-manager/config:go_default_library",
        "//vendor/k8s.io/controller-manager/pkg/features:go_default_library",
        "//vendor/k8s.io/controller-manager/pkg/features/register:go_default_library",
    ],
)
