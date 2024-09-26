load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "request",
    srcs = [
        "body_wrapper.go",
        "resp_writer_wrapper.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp/internal/request",
    importpath = "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp/internal/request",
    visibility = ["//vendor/go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp:__subpackages__"],
)
