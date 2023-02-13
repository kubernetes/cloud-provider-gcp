load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "aggregated",
    srcs = [
        "etag.go",
        "fake.go",
        "handler.go",
        "negotiation.go",
        "wrapper.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/endpoints/discovery/aggregated",
    importpath = "k8s.io/apiserver/pkg/endpoints/discovery/aggregated",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/emicklei/go-restful/v3:go-restful",
        "//vendor/github.com/google/go-cmp/cmp",
        "//vendor/k8s.io/api/apidiscovery/v2beta1",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
        "//vendor/k8s.io/apimachinery/pkg/runtime/serializer",
        "//vendor/k8s.io/apimachinery/pkg/util/runtime",
        "//vendor/k8s.io/apimachinery/pkg/util/wait",
        "//vendor/k8s.io/apimachinery/pkg/version",
        "//vendor/k8s.io/apiserver/pkg/endpoints/handlers/negotiation",
        "//vendor/k8s.io/apiserver/pkg/endpoints/handlers/responsewriters",
        "//vendor/k8s.io/apiserver/pkg/features",
        "//vendor/k8s.io/apiserver/pkg/util/feature",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
