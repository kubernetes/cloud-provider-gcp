load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "resolver",
    srcs = [
        "definitions.go",
        "discovery.go",
        "refs.go",
        "resolver.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/cel/openapi/resolver",
    importpath = "k8s.io/apiserver/pkg/cel/openapi/resolver",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
        "//vendor/k8s.io/apiserver/pkg/endpoints/openapi",
        "//vendor/k8s.io/client-go/discovery",
        "//vendor/k8s.io/kube-openapi/pkg/common",
        "//vendor/k8s.io/kube-openapi/pkg/validation/spec",
    ],
)
