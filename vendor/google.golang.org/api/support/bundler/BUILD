load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["bundler.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/google.golang.org/api/support/bundler",
    importpath = "google.golang.org/api/support/bundler",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/golang.org/x/net/context:go_default_library",
        "@org_golang_x_sync//semaphore:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = ["bundler_test.go"],
    embed = [":go_default_library"],
    deps = ["//vendor/golang.org/x/net/context:go_default_library"],
)
