load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "cel",
    srcs = [
        "compile.go",
        "interface.go",
        "mapper.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/authentication/cel",
    importpath = "k8s.io/apiserver/pkg/authentication/cel",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/google/cel-go/cel",
        "//vendor/github.com/google/cel-go/common/types/ref",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1/unstructured",
        "//vendor/k8s.io/apimachinery/pkg/util/version",
        "//vendor/k8s.io/apiserver/pkg/cel",
        "//vendor/k8s.io/apiserver/pkg/cel/environment",
    ],
)
