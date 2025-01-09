load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "direct",
    srcs = ["direct.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apimachinery/pkg/runtime/serializer/cbor/direct",
    importpath = "k8s.io/apimachinery/pkg/runtime/serializer/cbor/direct",
    visibility = ["//visibility:public"],
    deps = ["//vendor/k8s.io/apimachinery/pkg/runtime/serializer/cbor/internal/modes"],
)
