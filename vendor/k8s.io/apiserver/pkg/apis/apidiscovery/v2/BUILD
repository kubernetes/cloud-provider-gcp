load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "apidiscovery",
    srcs = [
        "conversion.go",
        "doc.go",
        "register.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/apis/apidiscovery/v2",
    importpath = "k8s.io/apiserver/pkg/apis/apidiscovery/v2",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/apidiscovery/v2:apidiscovery",
        "//vendor/k8s.io/api/apidiscovery/v2beta1",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/conversion",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
    ],
)
