load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["doc.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/cloud.google.com/go/cmd/go-cloud-debug-agent/internal/debug/tests/peek",
    importpath = "cloud.google.com/go/cmd/go-cloud-debug-agent/internal/debug/tests/peek",
    visibility = ["//vendor/cloud.google.com/go/cmd/go-cloud-debug-agent:__subpackages__"],
)

go_test(
    name = "go_default_test",
    srcs = ["peek_test.go"],
    embed = [":go_default_library"],
    deps = select({
        "@io_bazel_rules_go//go/platform:linux": [
            "//vendor/cloud.google.com/go/cmd/go-cloud-debug-agent/internal/debug:go_default_library",
            "//vendor/cloud.google.com/go/cmd/go-cloud-debug-agent/internal/debug/local:go_default_library",
            "//vendor/cloud.google.com/go/cmd/go-cloud-debug-agent/internal/debug/remote:go_default_library",
        ],
        "//conditions:default": [],
    }),
)
