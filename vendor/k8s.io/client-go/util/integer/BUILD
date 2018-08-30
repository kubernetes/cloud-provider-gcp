load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["integer.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/client-go/util/integer",
    importpath = "k8s.io/client-go/util/integer",
    visibility = ["//visibility:public"],
)

go_test(
    name = "go_default_test",
    srcs = ["integer_test.go"],
    embed = [":go_default_library"],
)
