load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["valuecollector.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/cloud.google.com/go/cmd/go-cloud-debug-agent/internal/valuecollector",
    importpath = "cloud.google.com/go/cmd/go-cloud-debug-agent/internal/valuecollector",
    visibility = ["//vendor/cloud.google.com/go/cmd/go-cloud-debug-agent:__subpackages__"],
    deps = [
        "//vendor/cloud.google.com/go/cmd/go-cloud-debug-agent/internal/debug:go_default_library",
        "//vendor/google.golang.org/api/clouddebugger/v2:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = ["valuecollector_test.go"],
    embed = [":go_default_library"],
    deps = [
        "//vendor/cloud.google.com/go/cmd/go-cloud-debug-agent/internal/debug:go_default_library",
        "//vendor/cloud.google.com/go/internal/testutil:go_default_library",
        "//vendor/google.golang.org/api/clouddebugger/v2:go_default_library",
    ],
)
