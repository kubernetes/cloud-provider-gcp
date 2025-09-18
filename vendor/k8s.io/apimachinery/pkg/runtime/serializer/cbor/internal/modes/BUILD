load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "modes",
    srcs = [
        "buffers.go",
        "decode.go",
        "diagnostic.go",
        "encode.go",
        "transcoding.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apimachinery/pkg/runtime/serializer/cbor/internal/modes",
    importpath = "k8s.io/apimachinery/pkg/runtime/serializer/cbor/internal/modes",
    visibility = ["//vendor/k8s.io/apimachinery/pkg/runtime/serializer/cbor:__subpackages__"],
    deps = [
        "//vendor/github.com/fxamacker/cbor/v2:cbor",
        "//vendor/sigs.k8s.io/json",
    ],
)
