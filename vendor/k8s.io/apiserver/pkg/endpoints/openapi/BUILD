load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "openapi",
    srcs = ["openapi.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/endpoints/openapi",
    importpath = "k8s.io/apiserver/pkg/endpoints/openapi",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/emicklei/go-restful/v3:go-restful",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
        "//vendor/k8s.io/kube-openapi/pkg/util",
        "//vendor/k8s.io/kube-openapi/pkg/validation/spec",
    ],
)
