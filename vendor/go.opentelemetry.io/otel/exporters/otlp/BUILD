load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "doc.go",
        "options.go",
        "optiontypes.go",
        "otlp.go",
        "protocoldriver.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel/exporters/otlp",
    importpath = "go.opentelemetry.io/otel/exporters/otlp",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/go.opentelemetry.io/otel:go_default_library",
        "//vendor/go.opentelemetry.io/otel/metric:go_default_library",
        "//vendor/go.opentelemetry.io/otel/sdk/export/metric:go_default_library",
        "//vendor/go.opentelemetry.io/otel/sdk/export/metric/aggregation:go_default_library",
        "//vendor/go.opentelemetry.io/otel/sdk/metric/controller/basic:go_default_library",
        "//vendor/go.opentelemetry.io/otel/sdk/metric/processor/basic:go_default_library",
        "//vendor/go.opentelemetry.io/otel/sdk/metric/selector/simple:go_default_library",
        "//vendor/go.opentelemetry.io/otel/sdk/trace:go_default_library",
    ],
)
