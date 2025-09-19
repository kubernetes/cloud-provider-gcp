load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "appsint",
    srcs = [
        "doc.go",
        "register.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/client-go/scale/scheme/appsint",
    importpath = "k8s.io/client-go/scale/scheme/appsint",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/apps/v1beta2",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
        "//vendor/k8s.io/client-go/scale/scheme",
    ],
)
