load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "defval",
    srcs = ["default.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/google.golang.org/protobuf/internal/encoding/defval",
    importpath = "google.golang.org/protobuf/internal/encoding/defval",
    visibility = ["//vendor/google.golang.org/protobuf:__subpackages__"],
    deps = [
        "//vendor/google.golang.org/protobuf/internal/encoding/text",
        "//vendor/google.golang.org/protobuf/internal/errors",
        "//vendor/google.golang.org/protobuf/reflect/protoreflect",
    ],
)
