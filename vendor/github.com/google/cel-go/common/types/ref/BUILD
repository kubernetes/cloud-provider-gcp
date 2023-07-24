load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "ref",
    srcs = [
        "provider.go",
        "reference.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/google/cel-go/common/types/ref",
    importpath = "github.com/google/cel-go/common/types/ref",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/google.golang.org/genproto/googleapis/api/expr/v1alpha1",
        "//vendor/google.golang.org/protobuf/proto",
        "//vendor/google.golang.org/protobuf/reflect/protoreflect",
    ],
)
