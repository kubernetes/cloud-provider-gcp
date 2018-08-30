load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["s3.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/github.com/docker/distribution/registry/storage/driver/s3-goamz",
    importpath = "github.com/docker/distribution/registry/storage/driver/s3-goamz",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/docker/distribution/context:go_default_library",
        "//vendor/github.com/docker/distribution/registry/client/transport:go_default_library",
        "//vendor/github.com/docker/distribution/registry/storage/driver:go_default_library",
        "//vendor/github.com/docker/distribution/registry/storage/driver/base:go_default_library",
        "//vendor/github.com/docker/distribution/registry/storage/driver/factory:go_default_library",
        "@com_github_docker_goamz//aws:go_default_library",
        "@com_github_docker_goamz//s3:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = ["s3_test.go"],
    embed = [":go_default_library"],
    deps = [
        "//vendor/github.com/docker/distribution/context:go_default_library",
        "//vendor/github.com/docker/distribution/registry/storage/driver:go_default_library",
        "//vendor/github.com/docker/distribution/registry/storage/driver/testsuites:go_default_library",
        "@com_github_docker_goamz//aws:go_default_library",
        "@com_github_docker_goamz//s3:go_default_library",
        "@in_gopkg_check_v1//:go_default_library",
    ],
)
