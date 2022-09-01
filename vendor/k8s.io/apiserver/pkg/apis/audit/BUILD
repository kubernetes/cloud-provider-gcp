load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "audit",
    srcs = [
        "doc.go",
        "helpers.go",
        "register.go",
        "types.go",
        "zz_generated.deepcopy.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/apis/audit",
    importpath = "k8s.io/apiserver/pkg/apis/audit",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/authentication/v1:authentication",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
        "//vendor/k8s.io/apimachinery/pkg/types",
    ],
)
