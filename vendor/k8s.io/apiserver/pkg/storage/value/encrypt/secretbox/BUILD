load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "secretbox",
    srcs = ["secretbox.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/storage/value/encrypt/secretbox",
    importpath = "k8s.io/apiserver/pkg/storage/value/encrypt/secretbox",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/golang.org/x/crypto/nacl/secretbox",
        "//vendor/k8s.io/apiserver/pkg/storage/value",
    ],
)
