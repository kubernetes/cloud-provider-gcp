load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["helloworld.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/google.golang.org/appengine/demos/helloworld",
    importpath = "google.golang.org/appengine/demos/helloworld",
    visibility = ["//visibility:private"],
    deps = [
        "//vendor/google.golang.org/appengine:go_default_library",
        "//vendor/google.golang.org/appengine/log:go_default_library",
    ],
)

go_binary(
    name = "helloworld",
    embed = [":go_default_library"],
    visibility = ["//visibility:public"],
)
