load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "context.go",
        "doc.go",
        "http.go",
        "logger.go",
        "trace.go",
        "util.go",
        "version.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/github.com/docker/distribution/context",
    importpath = "github.com/docker/distribution/context",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/docker/distribution/uuid:go_default_library",
        "//vendor/golang.org/x/net/context:go_default_library",
        "@com_github_gorilla_mux//:go_default_library",
        "@com_github_sirupsen_logrus//:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = [
        "http_test.go",
        "trace_test.go",
        "version_test.go",
    ],
    embed = [":go_default_library"],
)
