load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "protodelim",
    srcs = ["protodelim.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/google.golang.org/protobuf/encoding/protodelim",
    importpath = "google.golang.org/protobuf/encoding/protodelim",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/google.golang.org/protobuf/encoding/protowire",
        "//vendor/google.golang.org/protobuf/internal/errors",
        "//vendor/google.golang.org/protobuf/proto",
    ],
)
