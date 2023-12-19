load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "service",
    srcs = [
        "grpc_service.go",
        "interface.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/kms/pkg/service",
    importpath = "k8s.io/kms/pkg/service",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/google.golang.org/grpc",
        "//vendor/k8s.io/kms/apis/v2:apis",
    ],
)
