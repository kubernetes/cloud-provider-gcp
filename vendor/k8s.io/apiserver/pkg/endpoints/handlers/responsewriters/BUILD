load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "responsewriters",
    srcs = [
        "doc.go",
        "errors.go",
        "status.go",
        "writers.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/endpoints/handlers/responsewriters",
    importpath = "k8s.io/apiserver/pkg/endpoints/handlers/responsewriters",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/go.opentelemetry.io/otel/attribute",
        "//vendor/k8s.io/apimachinery/pkg/api/errors",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
        "//vendor/k8s.io/apimachinery/pkg/util/runtime",
        "//vendor/k8s.io/apiserver/pkg/audit",
        "//vendor/k8s.io/apiserver/pkg/authorization/authorizer",
        "//vendor/k8s.io/apiserver/pkg/endpoints/handlers/negotiation",
        "//vendor/k8s.io/apiserver/pkg/endpoints/metrics",
        "//vendor/k8s.io/apiserver/pkg/endpoints/request",
        "//vendor/k8s.io/apiserver/pkg/features",
        "//vendor/k8s.io/apiserver/pkg/registry/rest",
        "//vendor/k8s.io/apiserver/pkg/storage",
        "//vendor/k8s.io/apiserver/pkg/util/feature",
        "//vendor/k8s.io/apiserver/pkg/util/flushwriter",
        "//vendor/k8s.io/apiserver/pkg/util/wsstream",
        "//vendor/k8s.io/component-base/tracing",
    ],
)
