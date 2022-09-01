load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "negotiation",
    srcs = [
        "doc.go",
        "errors.go",
        "negotiate.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/endpoints/handlers/negotiation",
    importpath = "k8s.io/apiserver/pkg/endpoints/handlers/negotiation",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/munnerz/goautoneg",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
    ],
)
