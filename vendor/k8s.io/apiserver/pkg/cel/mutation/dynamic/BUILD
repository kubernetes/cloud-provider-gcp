load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "dynamic",
    srcs = ["objects.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/cel/mutation/dynamic",
    importpath = "k8s.io/apiserver/pkg/cel/mutation/dynamic",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/google/cel-go/common/types",
        "//vendor/github.com/google/cel-go/common/types/ref",
        "//vendor/github.com/google/cel-go/common/types/traits",
        "//vendor/google.golang.org/protobuf/types/known/structpb",
    ],
)
