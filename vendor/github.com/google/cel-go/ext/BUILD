load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "ext",
    srcs = [
        "bindings.go",
        "encoders.go",
        "guards.go",
        "math.go",
        "native.go",
        "protos.go",
        "sets.go",
        "strings.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/google/cel-go/ext",
    importpath = "github.com/google/cel-go/ext",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/google/cel-go/cel",
        "//vendor/github.com/google/cel-go/checker/decls",
        "//vendor/github.com/google/cel-go/common",
        "//vendor/github.com/google/cel-go/common/types",
        "//vendor/github.com/google/cel-go/common/types/pb",
        "//vendor/github.com/google/cel-go/common/types/ref",
        "//vendor/github.com/google/cel-go/common/types/traits",
        "//vendor/github.com/google/cel-go/interpreter",
        "//vendor/golang.org/x/text/language",
        "//vendor/golang.org/x/text/message",
        "//vendor/google.golang.org/genproto/googleapis/api/expr/v1alpha1",
        "//vendor/google.golang.org/protobuf/proto",
        "//vendor/google.golang.org/protobuf/reflect/protoreflect",
        "//vendor/google.golang.org/protobuf/types/known/structpb",
    ],
)
