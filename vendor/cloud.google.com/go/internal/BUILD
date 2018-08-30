load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "annotate.go",
        "retry.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/cloud.google.com/go/internal",
    importpath = "cloud.google.com/go/internal",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/golang.org/x/net/context:go_default_library",
        "//vendor/google.golang.org/api/googleapi:go_default_library",
        "@com_github_googleapis_gax_go//:go_default_library",
        "@org_golang_google_grpc//status:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = [
        "annotate_test.go",
        "retry_test.go",
    ],
    embed = [":go_default_library"],
    deps = [
        "//vendor/golang.org/x/net/context:go_default_library",
        "//vendor/google.golang.org/api/googleapi:go_default_library",
        "@com_github_googleapis_gax_go//:go_default_library",
        "@org_golang_google_grpc//codes:go_default_library",
        "@org_golang_google_grpc//status:go_default_library",
    ],
)
