load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "status",
    srcs = ["status.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/google.golang.org/grpc/status",
    importpath = "google.golang.org/grpc/status",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/google.golang.org/genproto/googleapis/rpc/status",
        "//vendor/google.golang.org/grpc/codes",
        "//vendor/google.golang.org/grpc/internal/status",
    ],
)
