load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["output.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/golang.org/x/text/internal/cldrtree/testdata/test2",
    importpath = "golang.org/x/text/internal/cldrtree/testdata/test2",
    visibility = ["//vendor/golang.org/x/text:__subpackages__"],
    deps = ["//vendor/golang.org/x/text/internal/cldrtree:go_default_library"],
)
