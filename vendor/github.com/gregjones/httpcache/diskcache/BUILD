load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["diskcache.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/github.com/gregjones/httpcache/diskcache",
    importpath = "github.com/gregjones/httpcache/diskcache",
    visibility = ["//visibility:public"],
    deps = ["//vendor/github.com/peterbourgon/diskv:go_default_library"],
)

go_test(
    name = "go_default_test",
    srcs = ["diskcache_test.go"],
    embed = [":go_default_library"],
)
