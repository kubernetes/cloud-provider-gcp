load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "descriptors.go",
        "doc.go",
        "errors.go",
        "headerparser.go",
        "routes.go",
        "urls.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/github.com/docker/distribution/registry/api/v2",
    importpath = "github.com/docker/distribution/registry/api/v2",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/docker/distribution/reference:go_default_library",
        "//vendor/github.com/docker/distribution/registry/api/errcode:go_default_library",
        "//vendor/github.com/opencontainers/go-digest:go_default_library",
        "@com_github_gorilla_mux//:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = [
        "headerparser_test.go",
        "routes_test.go",
        "urls_test.go",
    ],
    embed = [":go_default_library"],
    deps = [
        "//vendor/github.com/docker/distribution/reference:go_default_library",
        "@com_github_gorilla_mux//:go_default_library",
    ],
)
