load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "field",
    srcs = [
        "errors.go",
        "path.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apimachinery/pkg/util/validation/field",
    importpath = "k8s.io/apimachinery/pkg/util/validation/field",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/util/errors",
        "//vendor/k8s.io/apimachinery/pkg/util/sets",
    ],
)
