load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "synctrack",
    srcs = [
        "lazy.go",
        "synctrack.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/client-go/tools/cache/synctrack",
    importpath = "k8s.io/client-go/tools/cache/synctrack",
    visibility = ["//visibility:public"],
    deps = ["//vendor/k8s.io/apimachinery/pkg/util/sets"],
)
