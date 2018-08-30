load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "builder.go",
        "manifest.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/github.com/docker/distribution/manifest/schema2",
    importpath = "github.com/docker/distribution/manifest/schema2",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/docker/distribution:go_default_library",
        "//vendor/github.com/docker/distribution/context:go_default_library",
        "//vendor/github.com/docker/distribution/manifest:go_default_library",
        "//vendor/github.com/opencontainers/go-digest:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = [
        "builder_test.go",
        "manifest_test.go",
    ],
    embed = [":go_default_library"],
    deps = [
        "//vendor/github.com/docker/distribution:go_default_library",
        "//vendor/github.com/docker/distribution/context:go_default_library",
        "//vendor/github.com/opencontainers/go-digest:go_default_library",
    ],
)
