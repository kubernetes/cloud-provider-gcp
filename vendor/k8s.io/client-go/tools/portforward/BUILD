load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "portforward",
    srcs = [
        "doc.go",
        "fallback_dialer.go",
        "portforward.go",
        "tunneling_connection.go",
        "tunneling_dialer.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/client-go/tools/portforward",
    importpath = "k8s.io/client-go/tools/portforward",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/gorilla/websocket",
        "//vendor/k8s.io/api/core/v1:core",
        "//vendor/k8s.io/apimachinery/pkg/util/httpstream",
        "//vendor/k8s.io/apimachinery/pkg/util/httpstream/spdy",
        "//vendor/k8s.io/apimachinery/pkg/util/portforward",
        "//vendor/k8s.io/apimachinery/pkg/util/runtime",
        "//vendor/k8s.io/client-go/rest",
        "//vendor/k8s.io/client-go/transport/websocket",
        "//vendor/k8s.io/klog/v2:klog",
        "//vendor/k8s.io/utils/net",
    ],
)
