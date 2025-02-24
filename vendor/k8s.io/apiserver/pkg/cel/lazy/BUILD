load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "lazy",
    srcs = ["lazy.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/cel/lazy",
    importpath = "k8s.io/apiserver/pkg/cel/lazy",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/google/cel-go/common/types",
        "//vendor/github.com/google/cel-go/common/types/ref",
        "//vendor/github.com/google/cel-go/common/types/traits",
        "//vendor/k8s.io/apiserver/pkg/cel",
    ],
)
