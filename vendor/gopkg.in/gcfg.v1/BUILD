load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "doc.go",
        "errors.go",
        "read.go",
        "set.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/gopkg.in/gcfg.v1",
    importpath = "gopkg.in/gcfg.v1",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/gopkg.in/gcfg.v1/scanner:go_default_library",
        "//vendor/gopkg.in/gcfg.v1/token:go_default_library",
        "//vendor/gopkg.in/gcfg.v1/types:go_default_library",
        "//vendor/gopkg.in/warnings.v0:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = [
        "example_test.go",
        "issues_test.go",
        "read_test.go",
    ],
    data = glob(["testdata/**"]),
    embed = [":go_default_library"],
)
