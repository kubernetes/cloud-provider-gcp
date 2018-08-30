load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["cloud.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/cloud.google.com/go",
    importpath = "cloud.google.com/go",
    visibility = ["//visibility:public"],
)

go_test(
    name = "go_default_test",
    srcs = [
        "authexample_test.go",
        "examples_test.go",
        "import_test.go",
        "license_test.go",
    ],
    embed = [":go_default_library"],
    deps = [
        "//vendor/cloud.google.com/go/bigquery:go_default_library",
        "//vendor/cloud.google.com/go/datastore:go_default_library",
        "//vendor/cloud.google.com/go/pubsub:go_default_library",
        "//vendor/golang.org/x/net/context:go_default_library",
        "//vendor/golang.org/x/oauth2/google:go_default_library",
        "//vendor/google.golang.org/api/option:go_default_library",
    ],
)
