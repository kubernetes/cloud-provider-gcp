load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "main.go",
        "utils.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/gopkg.in/square/go-jose.v2/jose-util",
    importpath = "gopkg.in/square/go-jose.v2/jose-util",
    visibility = ["//visibility:private"],
    deps = [
        "//vendor/gopkg.in/square/go-jose.v2:go_default_library",
        "@in_gopkg_alecthomas_kingpin_v2//:go_default_library",
    ],
)

go_binary(
    name = "jose-util",
    embed = [":go_default_library"],
    visibility = ["//visibility:public"],
)
