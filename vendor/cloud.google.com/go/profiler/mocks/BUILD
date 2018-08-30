load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["mock_profiler_client.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/cloud.google.com/go/profiler/mocks",
    importpath = "cloud.google.com/go/profiler/mocks",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/golang.org/x/net/context:go_default_library",
        "@com_github_golang_mock//gomock:go_default_library",
        "@org_golang_google_genproto//googleapis/devtools/cloudprofiler/v2:go_default_library",
        "@org_golang_google_grpc//:go_default_library",
    ],
)
