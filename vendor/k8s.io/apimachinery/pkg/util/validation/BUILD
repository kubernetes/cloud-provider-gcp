load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["validation.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/apimachinery/pkg/util/validation",
    importpath = "k8s.io/apimachinery/pkg/util/validation",
    visibility = ["//visibility:public"],
    deps = ["//vendor/k8s.io/apimachinery/pkg/util/validation/field:go_default_library"],
)

go_test(
    name = "go_default_test",
    srcs = ["validation_test.go"],
    embed = [":go_default_library"],
    deps = ["//vendor/k8s.io/apimachinery/pkg/util/validation/field:go_default_library"],
)
