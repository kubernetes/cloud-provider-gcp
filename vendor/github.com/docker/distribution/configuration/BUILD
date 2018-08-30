load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "configuration.go",
        "parser.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/github.com/docker/distribution/configuration",
    importpath = "github.com/docker/distribution/configuration",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/gopkg.in/yaml.v2:go_default_library",
        "@com_github_sirupsen_logrus//:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = ["configuration_test.go"],
    embed = [":go_default_library"],
    deps = [
        "//vendor/gopkg.in/yaml.v2:go_default_library",
        "@in_gopkg_check_v1//:go_default_library",
    ],
)
