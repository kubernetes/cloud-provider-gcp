load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "metrics",
    srcs = ["metrics.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/endpoints/metrics",
    importpath = "k8s.io/apiserver/pkg/endpoints/metrics",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/emicklei/go-restful/v3:go-restful",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/internalversion",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1/validation",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
        "//vendor/k8s.io/apimachinery/pkg/types",
        "//vendor/k8s.io/apimachinery/pkg/util/runtime",
        "//vendor/k8s.io/apimachinery/pkg/util/sets",
        "//vendor/k8s.io/apiserver/pkg/audit",
        "//vendor/k8s.io/apiserver/pkg/authentication/user",
        "//vendor/k8s.io/apiserver/pkg/endpoints/request",
        "//vendor/k8s.io/apiserver/pkg/endpoints/responsewriter",
        "//vendor/k8s.io/component-base/metrics",
        "//vendor/k8s.io/component-base/metrics/legacyregistry",
    ],
)
