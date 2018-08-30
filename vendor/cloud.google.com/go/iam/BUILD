load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["iam.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/cloud.google.com/go/iam",
    importpath = "cloud.google.com/go/iam",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/golang.org/x/net/context:go_default_library",
        "@com_github_googleapis_gax_go//:go_default_library",
        "@org_golang_google_genproto//googleapis/iam/v1:go_default_library",
        "@org_golang_google_grpc//:go_default_library",
        "@org_golang_google_grpc//codes:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = ["iam_test.go"],
    embed = [":go_default_library"],
    deps = ["//vendor/cloud.google.com/go/internal/testutil:go_default_library"],
)
