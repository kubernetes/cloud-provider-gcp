load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "envelope",
    srcs = [
        "envelope.go",
        "grpc_service.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/storage/value/encrypt/envelope",
    importpath = "k8s.io/apiserver/pkg/storage/value/encrypt/envelope",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/golang.org/x/crypto/cryptobyte",
        "//vendor/google.golang.org/grpc",
        "//vendor/google.golang.org/grpc/credentials/insecure",
        "//vendor/k8s.io/apiserver/pkg/storage/value",
        "//vendor/k8s.io/apiserver/pkg/storage/value/encrypt/envelope/metrics",
        "//vendor/k8s.io/apiserver/pkg/storage/value/encrypt/envelope/util",
        "//vendor/k8s.io/apiserver/pkg/storage/value/encrypt/envelope/v1beta1",
        "//vendor/k8s.io/klog/v2:klog",
        "//vendor/k8s.io/utils/lru",
    ],
)
