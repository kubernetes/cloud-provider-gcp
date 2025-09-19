load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "extensionsint",
    srcs = [
        "doc.go",
        "register.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/client-go/scale/scheme/extensionsint",
    importpath = "k8s.io/client-go/scale/scheme/extensionsint",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/extensions/v1beta1",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
        "//vendor/k8s.io/client-go/scale/scheme",
    ],
)
