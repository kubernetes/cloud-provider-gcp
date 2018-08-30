load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "handler.go",
        "manifests.go",
        "tarfile.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/github.com/docker/distribution/testutil",
    importpath = "github.com/docker/distribution/testutil",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/docker/distribution:go_default_library",
        "//vendor/github.com/docker/distribution/context:go_default_library",
        "//vendor/github.com/docker/distribution/manifest:go_default_library",
        "//vendor/github.com/docker/distribution/manifest/manifestlist:go_default_library",
        "//vendor/github.com/docker/distribution/manifest/schema1:go_default_library",
        "//vendor/github.com/docker/distribution/manifest/schema2:go_default_library",
        "//vendor/github.com/opencontainers/go-digest:go_default_library",
        "@com_github_docker_libtrust//:go_default_library",
    ],
)
