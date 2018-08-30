load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["internal.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/golang.org/x/text/encoding/internal",
    importpath = "golang.org/x/text/encoding/internal",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/golang.org/x/text/encoding:go_default_library",
        "//vendor/golang.org/x/text/encoding/internal/identifier:go_default_library",
        "//vendor/golang.org/x/text/transform:go_default_library",
    ],
)
