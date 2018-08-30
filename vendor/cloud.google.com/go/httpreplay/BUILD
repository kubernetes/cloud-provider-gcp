load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["httpreplay.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/cloud.google.com/go/httpreplay",
    importpath = "cloud.google.com/go/httpreplay",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/cloud.google.com/go/httpreplay/internal/proxy:go_default_library",
        "//vendor/golang.org/x/net/context:go_default_library",
        "//vendor/google.golang.org/api/option:go_default_library",
        "//vendor/google.golang.org/api/transport/http:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = ["httpreplay_test.go"],
    embed = [":go_default_library"],
    deps = [
        "//vendor/cloud.google.com/go/internal/testutil:go_default_library",
        "//vendor/cloud.google.com/go/storage:go_default_library",
        "//vendor/golang.org/x/net/context:go_default_library",
        "//vendor/google.golang.org/api/option:go_default_library",
    ],
)
