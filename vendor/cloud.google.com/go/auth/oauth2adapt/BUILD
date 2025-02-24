load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "oauth2adapt",
    srcs = ["oauth2adapt.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/cloud.google.com/go/auth/oauth2adapt",
    importpath = "cloud.google.com/go/auth/oauth2adapt",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/cloud.google.com/go/auth",
        "//vendor/golang.org/x/oauth2",
        "//vendor/golang.org/x/oauth2/google",
    ],
)
