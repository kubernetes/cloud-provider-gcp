load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["trace.pb.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/go.opentelemetry.io/proto/otlp/trace/v1",
    importpath = "go.opentelemetry.io/proto/otlp/trace/v1",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/golang/protobuf/proto:go_default_library",
        "//vendor/go.opentelemetry.io/proto/otlp/common/v1:go_default_library",
        "//vendor/go.opentelemetry.io/proto/otlp/resource/v1:go_default_library",
        "//vendor/google.golang.org/protobuf/reflect/protoreflect:go_default_library",
        "//vendor/google.golang.org/protobuf/runtime/protoimpl:go_default_library",
    ],
)
