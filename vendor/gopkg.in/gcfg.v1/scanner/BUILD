load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "errors.go",
        "scanner.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/gopkg.in/gcfg.v1/scanner",
    importpath = "gopkg.in/gcfg.v1/scanner",
    visibility = ["//visibility:public"],
    deps = ["//vendor/gopkg.in/gcfg.v1/token:go_default_library"],
)

go_test(
    name = "go_default_test",
    srcs = [
        "example_test.go",
        "scanner_test.go",
    ],
    embed = [":go_default_library"],
    deps = ["//vendor/gopkg.in/gcfg.v1/token:go_default_library"],
)
