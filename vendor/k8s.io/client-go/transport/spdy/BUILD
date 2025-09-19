load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "spdy",
    srcs = ["spdy.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/client-go/transport/spdy",
    importpath = "k8s.io/client-go/transport/spdy",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/util/httpstream",
        "//vendor/k8s.io/apimachinery/pkg/util/httpstream/spdy",
        "//vendor/k8s.io/client-go/rest",
    ],
)
