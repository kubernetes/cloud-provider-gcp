load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "option",
    srcs = ["option.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/google.golang.org/api/option",
    importpath = "google.golang.org/api/option",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/cloud.google.com/go/auth",
        "//vendor/golang.org/x/oauth2",
        "//vendor/golang.org/x/oauth2/google",
        "//vendor/google.golang.org/api/internal",
        "//vendor/google.golang.org/api/internal/impersonate",
        "//vendor/google.golang.org/grpc",
    ],
)
