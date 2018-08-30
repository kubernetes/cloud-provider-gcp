load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["disco.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/google.golang.org/api/google-api-go-generator/internal/disco",
    importpath = "google.golang.org/api/google-api-go-generator/internal/disco",
    visibility = ["//vendor/google.golang.org/api/google-api-go-generator:__subpackages__"],
)

go_test(
    name = "go_default_test",
    srcs = ["disco_test.go"],
    data = glob(["testdata/**"]),
    embed = [":go_default_library"],
)
