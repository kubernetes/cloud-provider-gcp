load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "position.go",
        "serialize.go",
        "token.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/gopkg.in/gcfg.v1/token",
    importpath = "gopkg.in/gcfg.v1/token",
    visibility = ["//visibility:public"],
)

go_test(
    name = "go_default_test",
    srcs = [
        "position_test.go",
        "serialize_test.go",
    ],
    embed = [":go_default_library"],
)
