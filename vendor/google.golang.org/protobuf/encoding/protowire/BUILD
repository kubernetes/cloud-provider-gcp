load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "protowire",
    srcs = ["wire.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/google.golang.org/protobuf/encoding/protowire",
    importpath = "google.golang.org/protobuf/encoding/protowire",
    visibility = ["//visibility:public"],
    deps = ["//vendor/google.golang.org/protobuf/internal/errors"],
)
