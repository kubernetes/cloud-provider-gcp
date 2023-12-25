load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "errors",
    srcs = [
        "doc.go",
        "statuserror.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/admission/plugin/webhook/errors",
    importpath = "k8s.io/apiserver/pkg/admission/plugin/webhook/errors",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/api/errors",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
    ],
)
