load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "oauth2",
    srcs = [
        "deviceauth.go",
        "oauth2.go",
        "pkce.go",
        "token.go",
        "transport.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/golang.org/x/oauth2",
    importpath = "golang.org/x/oauth2",
    visibility = ["//visibility:public"],
    deps = ["//vendor/golang.org/x/oauth2/internal"],
)
