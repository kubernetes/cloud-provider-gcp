load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "protoiface",
    srcs = [
        "legacy.go",
        "methods.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/google.golang.org/protobuf/runtime/protoiface",
    importpath = "google.golang.org/protobuf/runtime/protoiface",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/google.golang.org/protobuf/internal/pragma",
        "//vendor/google.golang.org/protobuf/reflect/protoreflect",
    ],
)
