load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "googleapi.go",
        "types.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/google.golang.org/api/googleapi",
    importpath = "google.golang.org/api/googleapi",
    visibility = ["//visibility:public"],
    deps = ["//vendor/google.golang.org/api/googleapi/internal/uritemplates:go_default_library"],
)

go_test(
    name = "go_default_test",
    srcs = [
        "googleapi_test.go",
        "types_test.go",
    ],
    embed = [":go_default_library"],
)
