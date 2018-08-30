load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "generated.pb.go",
        "group_version.go",
        "interfaces.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/apimachinery/pkg/runtime/schema",
    importpath = "k8s.io/apimachinery/pkg/runtime/schema",
    visibility = ["//visibility:public"],
    deps = ["//vendor/github.com/gogo/protobuf/proto:go_default_library"],
)

go_test(
    name = "go_default_test",
    srcs = ["group_version_test.go"],
    embed = [":go_default_library"],
)
