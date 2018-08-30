load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["image.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/google.golang.org/appengine/image",
    importpath = "google.golang.org/appengine/image",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/golang.org/x/net/context:go_default_library",
        "//vendor/google.golang.org/appengine:go_default_library",
        "//vendor/google.golang.org/appengine/internal:go_default_library",
        "//vendor/google.golang.org/appengine/internal/image:go_default_library",
    ],
)
