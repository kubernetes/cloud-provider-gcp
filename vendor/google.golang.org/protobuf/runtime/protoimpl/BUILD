load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "protoimpl",
    srcs = [
        "impl.go",
        "version.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/google.golang.org/protobuf/runtime/protoimpl",
    importpath = "google.golang.org/protobuf/runtime/protoimpl",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/google.golang.org/protobuf/internal/filedesc",
        "//vendor/google.golang.org/protobuf/internal/filetype",
        "//vendor/google.golang.org/protobuf/internal/impl",
        "//vendor/google.golang.org/protobuf/internal/protolazy",
        "//vendor/google.golang.org/protobuf/internal/version",
    ],
)
