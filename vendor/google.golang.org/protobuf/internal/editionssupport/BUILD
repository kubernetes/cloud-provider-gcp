load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "editionssupport",
    srcs = ["editions.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/google.golang.org/protobuf/internal/editionssupport",
    importpath = "google.golang.org/protobuf/internal/editionssupport",
    visibility = ["//vendor/google.golang.org/protobuf:__subpackages__"],
    deps = ["//vendor/google.golang.org/protobuf/types/descriptorpb"],
)
