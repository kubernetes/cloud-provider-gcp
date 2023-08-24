load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "websocket",
    srcs = ["protocol.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/authentication/request/websocket",
    importpath = "k8s.io/apiserver/pkg/authentication/request/websocket",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/util/httpstream/wsstream",
        "//vendor/k8s.io/apiserver/pkg/authentication/authenticator",
    ],
)
