load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "envelope.go",
        "grpc_service.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/storage/value/encrypt/envelope",
    importpath = "k8s.io/apiserver/pkg/storage/value/encrypt/envelope",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/golang.org/x/crypto/cryptobyte:go_default_library",
        "//vendor/google.golang.org/grpc:go_default_library",
        "//vendor/google.golang.org/grpc/credentials/insecure:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/storage/value:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/storage/value/encrypt/envelope/metrics:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/storage/value/encrypt/envelope/util:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/storage/value/encrypt/envelope/v1beta1:go_default_library",
        "//vendor/k8s.io/klog/v2:go_default_library",
        "//vendor/k8s.io/utils/lru:go_default_library",
    ],
)
