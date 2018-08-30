load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["redis.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/github.com/docker/distribution/registry/storage/cache/redis",
    importpath = "github.com/docker/distribution/registry/storage/cache/redis",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/docker/distribution:go_default_library",
        "//vendor/github.com/docker/distribution/context:go_default_library",
        "//vendor/github.com/docker/distribution/reference:go_default_library",
        "//vendor/github.com/docker/distribution/registry/storage/cache:go_default_library",
        "//vendor/github.com/opencontainers/go-digest:go_default_library",
        "@com_github_garyburd_redigo//redis:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = ["redis_test.go"],
    embed = [":go_default_library"],
    deps = [
        "//vendor/github.com/docker/distribution/registry/storage/cache/cachecheck:go_default_library",
        "@com_github_garyburd_redigo//redis:go_default_library",
    ],
)
