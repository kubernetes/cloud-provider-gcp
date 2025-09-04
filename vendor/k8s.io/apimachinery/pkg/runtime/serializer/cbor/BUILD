load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "cbor",
    srcs = [
        "cbor.go",
        "framer.go",
        "raw.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apimachinery/pkg/runtime/serializer/cbor",
    importpath = "k8s.io/apimachinery/pkg/runtime/serializer/cbor",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/fxamacker/cbor/v2:cbor",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1/unstructured",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
        "//vendor/k8s.io/apimachinery/pkg/runtime/serializer/cbor/internal/modes",
        "//vendor/k8s.io/apimachinery/pkg/runtime/serializer/recognizer",
        "//vendor/k8s.io/apimachinery/pkg/util/runtime",
    ],
)
