load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["leveldbcache.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/github.com/gregjones/httpcache/leveldbcache",
    importpath = "github.com/gregjones/httpcache/leveldbcache",
    visibility = ["//visibility:public"],
    deps = ["@com_github_syndtr_goleveldb//leveldb:go_default_library"],
)

go_test(
    name = "go_default_test",
    srcs = ["leveldbcache_test.go"],
    embed = [":go_default_library"],
)
