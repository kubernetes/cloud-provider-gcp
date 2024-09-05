load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "dump",
    srcs = ["dump.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apimachinery/pkg/util/dump",
    importpath = "k8s.io/apimachinery/pkg/util/dump",
    visibility = ["//visibility:public"],
    deps = ["//vendor/github.com/davecgh/go-spew/spew"],
)
