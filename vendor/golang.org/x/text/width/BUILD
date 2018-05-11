load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "kind_string.go",
        "tables10.0.0.go",
        "tables9.0.0.go",
        "transform.go",
        "trieval.go",
        "width.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/golang.org/x/text/width",
    importpath = "golang.org/x/text/width",
    visibility = ["//visibility:public"],
    deps = ["//vendor/golang.org/x/text/transform:go_default_library"],
)
