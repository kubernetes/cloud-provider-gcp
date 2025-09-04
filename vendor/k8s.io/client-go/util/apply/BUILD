load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "apply",
    srcs = ["apply.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/client-go/util/apply",
    importpath = "k8s.io/client-go/util/apply",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/runtime/serializer/cbor/direct",
        "//vendor/k8s.io/apimachinery/pkg/types",
        "//vendor/k8s.io/apimachinery/pkg/util/json",
        "//vendor/k8s.io/client-go/features",
        "//vendor/k8s.io/client-go/rest",
    ],
)
