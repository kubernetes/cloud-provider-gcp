load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "accesscontroller.go",
        "stringset.go",
        "token.go",
        "util.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/github.com/docker/distribution/registry/auth/token",
    importpath = "github.com/docker/distribution/registry/auth/token",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/docker/distribution/context:go_default_library",
        "//vendor/github.com/docker/distribution/registry/auth:go_default_library",
        "@com_github_docker_libtrust//:go_default_library",
        "@com_github_sirupsen_logrus//:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = ["token_test.go"],
    embed = [":go_default_library"],
    deps = [
        "//vendor/github.com/docker/distribution/context:go_default_library",
        "//vendor/github.com/docker/distribution/registry/auth:go_default_library",
        "@com_github_docker_libtrust//:go_default_library",
    ],
)
