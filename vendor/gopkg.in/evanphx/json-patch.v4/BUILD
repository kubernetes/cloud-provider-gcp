load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "json-patch_v4",
    srcs = [
        "errors.go",
        "merge.go",
        "patch.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/gopkg.in/evanphx/json-patch.v4",
    importpath = "gopkg.in/evanphx/json-patch.v4",
    visibility = ["//visibility:public"],
    deps = ["//vendor/github.com/pkg/errors"],
)
