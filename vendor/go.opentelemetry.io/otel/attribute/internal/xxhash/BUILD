load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "xxhash",
    srcs = ["xxhash.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/otel/attribute/internal/xxhash",
    importpath = "go.opentelemetry.io/otel/attribute/internal/xxhash",
    visibility = ["//vendor/go.opentelemetry.io/otel/attribute:__subpackages__"],
    deps = ["//vendor/github.com/cespare/xxhash/v2:xxhash"],
)
