load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "pbutil",
    srcs = [
        "decode.go",
        "doc.go",
        "encode.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/matttproud/golang_protobuf_extensions/v2/pbutil",
    importpath = "github.com/matttproud/golang_protobuf_extensions/v2/pbutil",
    visibility = ["//visibility:public"],
    deps = ["//vendor/google.golang.org/protobuf/proto"],
)
