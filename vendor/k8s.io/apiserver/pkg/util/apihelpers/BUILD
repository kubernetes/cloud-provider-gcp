load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "apihelpers",
    srcs = ["helpers.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/util/apihelpers",
    importpath = "k8s.io/apiserver/pkg/util/apihelpers",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/flowcontrol/v1:flowcontrol",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/internalversion/scheme",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/runtime/serializer",
        "//vendor/k8s.io/apimachinery/pkg/runtime/serializer/cbor",
        "//vendor/k8s.io/apiserver/pkg/features",
        "//vendor/k8s.io/apiserver/pkg/util/feature",
    ],
)
