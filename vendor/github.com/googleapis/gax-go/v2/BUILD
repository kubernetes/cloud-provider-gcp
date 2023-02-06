load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "gax-go",
    srcs = [
        "call_option.go",
        "gax.go",
        "header.go",
        "invoke.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/googleapis/gax-go/v2",
    importpath = "github.com/googleapis/gax-go/v2",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/googleapis/gax-go/v2/apierror",
        "//vendor/google.golang.org/grpc",
        "//vendor/google.golang.org/grpc/codes",
        "//vendor/google.golang.org/grpc/status",
    ],
)
