load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["cbtconfig.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/cloud.google.com/go/bigtable/internal/cbtconfig",
    importpath = "cloud.google.com/go/bigtable/internal/cbtconfig",
    visibility = ["//vendor/cloud.google.com/go/bigtable:__subpackages__"],
    deps = [
        "//vendor/golang.org/x/oauth2:go_default_library",
        "@org_golang_google_grpc//credentials:go_default_library",
    ],
)
