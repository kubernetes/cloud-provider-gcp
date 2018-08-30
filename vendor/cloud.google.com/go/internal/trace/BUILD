load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "go18.go",
        "not_go18.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/cloud.google.com/go/internal/trace",
    importpath = "cloud.google.com/go/internal/trace",
    visibility = ["//vendor/cloud.google.com/go:__subpackages__"],
    deps = [
        "//vendor/golang.org/x/net/context:go_default_library",
        "//vendor/google.golang.org/api/googleapi:go_default_library",
        "@io_opencensus_go//trace:go_default_library",
        "@org_golang_google_genproto//googleapis/rpc/code:go_default_library",
        "@org_golang_google_grpc//status:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = ["go18_test.go"],
    embed = [":go_default_library"],
    deps = [
        "//vendor/cloud.google.com/go/internal/testutil:go_default_library",
        "//vendor/google.golang.org/api/googleapi:go_default_library",
        "@io_opencensus_go//trace:go_default_library",
        "@org_golang_google_genproto//googleapis/rpc/code:go_default_library",
        "@org_golang_google_grpc//codes:go_default_library",
        "@org_golang_google_grpc//status:go_default_library",
    ],
)
