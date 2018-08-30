load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["main.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/gopkg.in/square/go-jose.v2/jwk-keygen",
    importpath = "gopkg.in/square/go-jose.v2/jwk-keygen",
    visibility = ["//visibility:private"],
    deps = [
        "//vendor/golang.org/x/crypto/ed25519:go_default_library",
        "//vendor/gopkg.in/square/go-jose.v2:go_default_library",
        "@in_gopkg_alecthomas_kingpin_v2//:go_default_library",
    ],
)

go_binary(
    name = "jwk-keygen",
    embed = [":go_default_library"],
    visibility = ["//visibility:public"],
)
