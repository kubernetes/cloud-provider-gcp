load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "mergepatch",
    srcs = [
        "errors.go",
        "util.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apimachinery/pkg/util/mergepatch",
    importpath = "k8s.io/apimachinery/pkg/util/mergepatch",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/util/dump",
        "//vendor/sigs.k8s.io/yaml",
    ],
)
