load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "debug.go",
        "record.go",
        "replay.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/cloud.google.com/go/httpreplay/internal/proxy",
    importpath = "cloud.google.com/go/httpreplay/internal/proxy",
    visibility = ["//vendor/cloud.google.com/go/httpreplay:__subpackages__"],
    deps = [
        "@com_github_google_martian//:go_default_library",
        "@com_github_google_martian//fifo:go_default_library",
        "@com_github_google_martian//har:go_default_library",
        "@com_github_google_martian//httpspec:go_default_library",
        "@com_github_google_martian//martianlog:go_default_library",
        "@com_github_google_martian//mitm:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = [
        "record_test.go",
        "replay_test.go",
    ],
    embed = [":go_default_library"],
    deps = [
        "//vendor/cloud.google.com/go/internal/testutil:go_default_library",
        "@com_github_google_go_cmp//cmp:go_default_library",
        "@com_github_google_go_cmp//cmp/cmpopts:go_default_library",
        "@com_github_google_martian//:go_default_library",
        "@com_github_google_martian//har:go_default_library",
    ],
)
