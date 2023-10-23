load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "webhook",
    srcs = ["webhook.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/plugin/pkg/audit/webhook",
    importpath = "k8s.io/apiserver/plugin/pkg/audit/webhook",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/go.opentelemetry.io/otel/attribute",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
        "//vendor/k8s.io/apimachinery/pkg/util/net",
        "//vendor/k8s.io/apimachinery/pkg/util/wait",
        "//vendor/k8s.io/apiserver/pkg/apis/audit",
        "//vendor/k8s.io/apiserver/pkg/apis/audit/install",
        "//vendor/k8s.io/apiserver/pkg/audit",
        "//vendor/k8s.io/apiserver/pkg/util/webhook",
        "//vendor/k8s.io/client-go/rest",
        "//vendor/k8s.io/component-base/tracing",
    ],
)
