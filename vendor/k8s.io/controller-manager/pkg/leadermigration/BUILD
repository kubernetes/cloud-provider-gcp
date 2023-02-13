load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "leadermigration",
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
        "//vendor/k8s.io/apiserver/pkg/util/feature",
        "//vendor/k8s.io/controller-manager/config",
        "//vendor/k8s.io/controller-manager/pkg/features",
        "//vendor/k8s.io/controller-manager/pkg/features/register",
    ],
)
