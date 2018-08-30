load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["runtime.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/apimachinery/pkg/util/runtime",
    importpath = "k8s.io/apimachinery/pkg/util/runtime",
    visibility = ["//visibility:public"],
    deps = ["//vendor/github.com/golang/glog:go_default_library"],
)

go_test(
    name = "go_default_test",
    srcs = ["runtime_test.go"],
    embed = [":go_default_library"],
)
