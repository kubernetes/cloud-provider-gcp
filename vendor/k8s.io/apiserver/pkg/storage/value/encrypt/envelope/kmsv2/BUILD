load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "kmsv2",
    srcs = [
        "envelope.go",
        "grpc_service.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/storage/value/encrypt/envelope/kmsv2",
    importpath = "k8s.io/apiserver/pkg/storage/value/encrypt/envelope/kmsv2",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/gogo/protobuf/proto",
        "//vendor/google.golang.org/grpc",
        "//vendor/k8s.io/apimachinery/pkg/util/uuid",
        "//vendor/k8s.io/apiserver/pkg/storage/value",
        "//vendor/k8s.io/apiserver/pkg/storage/value/encrypt/envelope/kmsv2/v2alpha1",
        "//vendor/k8s.io/apiserver/pkg/storage/value/encrypt/envelope/metrics",
        "//vendor/k8s.io/apiserver/pkg/storage/value/encrypt/envelope/util",
        "//vendor/k8s.io/apiserver/pkg/storage/value/encrypt/envelope/v2alpha1",
        "//vendor/k8s.io/klog/v2:klog",
        "//vendor/k8s.io/utils/lru",
    ],
)
