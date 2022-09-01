load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "global",
    srcs = [
        "meter.go",
        "propagator.go",
        "state.go",
        "trace.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel/internal/global",
    importpath = "go.opentelemetry.io/otel/internal/global",
    visibility = ["//vendor/go.opentelemetry.io/otel:__subpackages__"],
    deps = [
        "//vendor/go.opentelemetry.io/otel/attribute",
        "//vendor/go.opentelemetry.io/otel/internal/trace/noop",
        "//vendor/go.opentelemetry.io/otel/metric",
        "//vendor/go.opentelemetry.io/otel/metric/number",
        "//vendor/go.opentelemetry.io/otel/metric/registry",
        "//vendor/go.opentelemetry.io/otel/propagation",
        "//vendor/go.opentelemetry.io/otel/trace",
    ],
)
