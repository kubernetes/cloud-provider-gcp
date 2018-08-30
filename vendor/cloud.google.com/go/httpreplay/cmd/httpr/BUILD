load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["httpr.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/cloud.google.com/go/httpreplay/cmd/httpr",
    importpath = "cloud.google.com/go/httpreplay/cmd/httpr",
    visibility = ["//visibility:private"],
    deps = [
        "//vendor/cloud.google.com/go/httpreplay/internal/proxy:go_default_library",
        "@com_github_google_martian//martianhttp:go_default_library",
    ],
)

go_binary(
    name = "httpr",
    embed = [":go_default_library"],
    visibility = ["//visibility:public"],
)

go_test(
    name = "go_default_test",
    srcs = ["integration_test.go"],
    embed = [":go_default_library"],
    deps = [
        "//vendor/cloud.google.com/go/internal/testutil:go_default_library",
        "//vendor/cloud.google.com/go/storage:go_default_library",
        "//vendor/golang.org/x/net/context:go_default_library",
        "//vendor/golang.org/x/oauth2:go_default_library",
        "//vendor/google.golang.org/api/option:go_default_library",
    ],
)
