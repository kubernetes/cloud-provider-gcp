load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "validation",
    srcs = ["validation.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/apis/audit/validation",
    importpath = "k8s.io/apiserver/pkg/apis/audit/validation",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/api/validation",
        "//vendor/k8s.io/apimachinery/pkg/util/validation/field",
        "//vendor/k8s.io/apiserver/pkg/apis/audit",
    ],
)
