load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "api_version.go",
        "session.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/github.com/docker/distribution/registry/client/auth",
    importpath = "github.com/docker/distribution/registry/client/auth",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/docker/distribution/registry/client:go_default_library",
        "//vendor/github.com/docker/distribution/registry/client/auth/challenge:go_default_library",
        "//vendor/github.com/docker/distribution/registry/client/transport:go_default_library",
        "@com_github_sirupsen_logrus//:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = ["session_test.go"],
    embed = [":go_default_library"],
    deps = [
        "//vendor/github.com/docker/distribution/registry/client/auth/challenge:go_default_library",
        "//vendor/github.com/docker/distribution/registry/client/transport:go_default_library",
        "//vendor/github.com/docker/distribution/testutil:go_default_library",
    ],
)
