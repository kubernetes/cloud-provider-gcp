load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["protocol.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/cloud.google.com/go/cmd/go-cloud-debug-agent/internal/debug/server/protocol",
    importpath = "cloud.google.com/go/cmd/go-cloud-debug-agent/internal/debug/server/protocol",
    visibility = ["//vendor/cloud.google.com/go/cmd/go-cloud-debug-agent:__subpackages__"],
    deps = ["//vendor/cloud.google.com/go/cmd/go-cloud-debug-agent/internal/debug:go_default_library"],
)
