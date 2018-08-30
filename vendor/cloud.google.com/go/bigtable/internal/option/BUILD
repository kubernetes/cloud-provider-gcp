load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["option.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/cloud.google.com/go/bigtable/internal/option",
    importpath = "cloud.google.com/go/bigtable/internal/option",
    visibility = ["//vendor/cloud.google.com/go/bigtable:__subpackages__"],
    deps = [
        "//vendor/google.golang.org/api/option:go_default_library",
        "@org_golang_google_grpc//:go_default_library",
    ],
)
