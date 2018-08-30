load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["jws.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/golang.org/x/oauth2/jws",
    importpath = "golang.org/x/oauth2/jws",
    visibility = ["//visibility:public"],
)

go_test(
    name = "go_default_test",
    srcs = ["jws_test.go"],
    embed = [":go_default_library"],
)
