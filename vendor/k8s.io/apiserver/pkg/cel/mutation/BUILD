load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "mutation",
    srcs = [
        "jsonpatch.go",
        "typeresolver.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/cel/mutation",
    importpath = "k8s.io/apiserver/pkg/cel/mutation",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/google/cel-go/cel",
        "//vendor/github.com/google/cel-go/common/types",
        "//vendor/github.com/google/cel-go/common/types/ref",
        "//vendor/k8s.io/apiserver/pkg/cel/common",
        "//vendor/k8s.io/apiserver/pkg/cel/mutation/dynamic",
    ],
)
