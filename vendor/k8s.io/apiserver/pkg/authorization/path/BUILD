load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "path",
    srcs = [
        "doc.go",
        "path.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/authorization/path",
    importpath = "k8s.io/apiserver/pkg/authorization/path",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/util/sets",
        "//vendor/k8s.io/apiserver/pkg/authorization/authorizer",
    ],
)
