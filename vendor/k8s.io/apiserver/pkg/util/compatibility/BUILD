load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "compatibility",
    srcs = [
        "registry.go",
        "version.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/util/compatibility",
    importpath = "k8s.io/apiserver/pkg/util/compatibility",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/util/runtime",
        "//vendor/k8s.io/apimachinery/pkg/util/version",
        "//vendor/k8s.io/apiserver/pkg/util/feature",
        "//vendor/k8s.io/component-base/compatibility",
        "//vendor/k8s.io/component-base/version",
    ],
)
