load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "v2alpha1",
    srcs = [
        "api.pb.go",
        "v2alpha1.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/kms/apis/v2alpha1",
    importpath = "k8s.io/kms/apis/v2alpha1",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/gogo/protobuf/proto",
        "//vendor/google.golang.org/grpc",
        "//vendor/google.golang.org/grpc/codes",
        "//vendor/google.golang.org/grpc/status",
    ],
)
