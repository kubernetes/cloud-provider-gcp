load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["test.pb.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/github.com/gogo/protobuf/protoc-gen-gogo/testdata/my_test",
    importpath = "github.com/gogo/protobuf/protoc-gen-gogo/testdata/my_test",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/gogo/protobuf/proto:go_default_library",
        "@com_github_gogo_protobuf//protoc-gen-gogo/testdata/multi:go_default_library",
    ],
)
