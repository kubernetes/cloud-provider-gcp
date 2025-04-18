load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "telemetry",
    srcs = [
        "attr.go",
        "doc.go",
        "id.go",
        "number.go",
        "resource.go",
        "scope.go",
        "span.go",
        "status.go",
        "traces.go",
        "value.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/auto/sdk/internal/telemetry",
    importpath = "go.opentelemetry.io/auto/sdk/internal/telemetry",
    visibility = ["//vendor/go.opentelemetry.io/auto/sdk:__subpackages__"],
)
