load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "api",
    srcs = [
        "attributes.go",
        "constants.go",
        "doc.go",
        "helpers.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/pod-security-admission/api",
    importpath = "k8s.io/pod-security-admission/api",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/admission/v1:admission",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
        "//vendor/k8s.io/apimachinery/pkg/util/validation/field",
        "//vendor/k8s.io/component-base/version",
    ],
)
