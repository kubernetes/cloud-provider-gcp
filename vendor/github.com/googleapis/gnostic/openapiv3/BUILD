load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "OpenAPIv3.go",
        "OpenAPIv3.pb.go",
        "document.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/googleapis/gnostic/openapiv3",
    importpath = "github.com/googleapis/gnostic/openapiv3",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/googleapis/gnostic/compiler:go_default_library",
        "//vendor/google.golang.org/protobuf/reflect/protoreflect:go_default_library",
        "//vendor/google.golang.org/protobuf/runtime/protoimpl:go_default_library",
        "//vendor/google.golang.org/protobuf/types/known/anypb:go_default_library",
        "//vendor/gopkg.in/yaml.v3:go_default_library",
    ],
)
