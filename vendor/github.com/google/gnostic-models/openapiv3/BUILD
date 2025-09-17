load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "openapiv3",
    srcs = [
        "OpenAPIv3.go",
        "OpenAPIv3.pb.go",
        "annotations.pb.go",
        "document.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/google/gnostic-models/openapiv3",
    importpath = "github.com/google/gnostic-models/openapiv3",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/google/gnostic-models/compiler",
        "//vendor/go.yaml.in/yaml/v3:yaml",
        "//vendor/google.golang.org/protobuf/reflect/protoreflect",
        "//vendor/google.golang.org/protobuf/runtime/protoimpl",
        "//vendor/google.golang.org/protobuf/types/descriptorpb",
        "//vendor/google.golang.org/protobuf/types/known/anypb",
    ],
)
