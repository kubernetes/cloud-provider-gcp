load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "cbt.go",
        "cbtdoc.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/cloud.google.com/go/bigtable/cmd/cbt",
    importpath = "cloud.google.com/go/bigtable/cmd/cbt",
    visibility = ["//visibility:private"],
    deps = [
        "//vendor/cloud.google.com/go/bigtable:go_default_library",
        "//vendor/cloud.google.com/go/bigtable/internal/cbtconfig:go_default_library",
        "//vendor/golang.org/x/net/context:go_default_library",
        "//vendor/google.golang.org/api/iterator:go_default_library",
        "//vendor/google.golang.org/api/option:go_default_library",
        "@org_golang_google_grpc//:go_default_library",
    ],
)

go_binary(
    name = "cbt",
    embed = [":go_default_library"],
    visibility = ["//visibility:public"],
)

go_test(
    name = "go_default_test",
    srcs = ["cbt_test.go"],
    embed = [":go_default_library"],
    deps = [
        "//vendor/cloud.google.com/go/bigtable:go_default_library",
        "//vendor/cloud.google.com/go/internal/testutil:go_default_library",
        "@com_github_google_go_cmp//cmp:go_default_library",
    ],
)
