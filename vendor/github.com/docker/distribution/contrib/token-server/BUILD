load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "errors.go",
        "main.go",
        "token.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/github.com/docker/distribution/contrib/token-server",
    importpath = "github.com/docker/distribution/contrib/token-server",
    visibility = ["//visibility:private"],
    deps = [
        "//vendor/github.com/docker/distribution/context:go_default_library",
        "//vendor/github.com/docker/distribution/registry/api/errcode:go_default_library",
        "//vendor/github.com/docker/distribution/registry/auth:go_default_library",
        "//vendor/github.com/docker/distribution/registry/auth/htpasswd:go_default_library",
        "//vendor/github.com/docker/distribution/registry/auth/token:go_default_library",
        "@com_github_docker_libtrust//:go_default_library",
        "@com_github_gorilla_mux//:go_default_library",
        "@com_github_sirupsen_logrus//:go_default_library",
    ],
)

go_binary(
    name = "token-server",
    embed = [":go_default_library"],
    visibility = ["//visibility:public"],
)
