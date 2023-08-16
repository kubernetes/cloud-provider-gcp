load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "httprule",
    srcs = [
        "compile.go",
        "parse.go",
        "types.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/grpc-ecosystem/grpc-gateway/v2/internal/httprule",
    importpath = "github.com/grpc-ecosystem/grpc-gateway/v2/internal/httprule",
    visibility = ["//vendor/github.com/grpc-ecosystem/grpc-gateway/v2:__subpackages__"],
    deps = ["//vendor/github.com/grpc-ecosystem/grpc-gateway/v2/utilities"],
)
