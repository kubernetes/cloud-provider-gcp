load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "editiondefaults",
    srcs = ["defaults.go"],
    embedsrcs = ["editions_defaults.binpb"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/google.golang.org/protobuf/internal/editiondefaults",
    importpath = "google.golang.org/protobuf/internal/editiondefaults",
    visibility = ["//vendor/google.golang.org/protobuf:__subpackages__"],
)
