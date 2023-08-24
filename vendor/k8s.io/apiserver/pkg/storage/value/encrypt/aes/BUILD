load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "aes",
    srcs = [
        "aes.go",
        "aes_extended_nonce.go",
        "cache.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/storage/value/encrypt/aes",
    importpath = "k8s.io/apiserver/pkg/storage/value/encrypt/aes",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/golang.org/x/crypto/hkdf",
        "//vendor/k8s.io/apimachinery/pkg/util/cache",
        "//vendor/k8s.io/apiserver/pkg/storage/value",
        "//vendor/k8s.io/klog/v2:klog",
        "//vendor/k8s.io/utils/clock",
    ],
)
