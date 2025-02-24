load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "compiler",
    srcs = [
        "context.go",
        "error.go",
        "extensions.go",
        "helpers.go",
        "main.go",
        "reader.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/google/gnostic-models/compiler",
    importpath = "github.com/google/gnostic-models/compiler",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/google/gnostic-models/extensions",
        "//vendor/github.com/google/gnostic-models/jsonschema",
        "//vendor/google.golang.org/protobuf/proto",
        "//vendor/google.golang.org/protobuf/types/known/anypb",
        "//vendor/gopkg.in/yaml.v3:yaml_v3",
    ],
)
