load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["googleauth.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/github.com/googleapis/gnostic/plugins/gnostic-go-generator/examples/googleauth",
    importpath = "github.com/googleapis/gnostic/plugins/gnostic-go-generator/examples/googleauth",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/golang.org/x/net/context:go_default_library",
        "//vendor/golang.org/x/oauth2:go_default_library",
    ],
)
