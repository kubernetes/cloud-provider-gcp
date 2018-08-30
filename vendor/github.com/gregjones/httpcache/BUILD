load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["httpcache.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/github.com/gregjones/httpcache",
    importpath = "github.com/gregjones/httpcache",
    visibility = ["//visibility:public"],
)

go_test(
    name = "go_default_test",
    srcs = ["httpcache_test.go"],
    embed = [":go_default_library"],
)
