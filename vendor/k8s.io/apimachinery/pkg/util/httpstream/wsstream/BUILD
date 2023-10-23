load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "wsstream",
    srcs = [
        "conn.go",
        "doc.go",
        "stream.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apimachinery/pkg/util/httpstream/wsstream",
    importpath = "k8s.io/apimachinery/pkg/util/httpstream/wsstream",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/golang.org/x/net/websocket",
        "//vendor/k8s.io/apimachinery/pkg/util/runtime",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
