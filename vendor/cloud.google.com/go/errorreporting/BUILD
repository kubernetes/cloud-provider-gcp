load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["errors.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/cloud.google.com/go/errorreporting",
    importpath = "cloud.google.com/go/errorreporting",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/cloud.google.com/go/errorreporting/apiv1beta1:go_default_library",
        "//vendor/cloud.google.com/go/internal/version:go_default_library",
        "//vendor/github.com/golang/protobuf/ptypes:go_default_library",
        "//vendor/golang.org/x/net/context:go_default_library",
        "//vendor/google.golang.org/api/option:go_default_library",
        "//vendor/google.golang.org/api/support/bundler:go_default_library",
        "@com_github_googleapis_gax_go//:go_default_library",
        "@org_golang_google_genproto//googleapis/devtools/clouderrorreporting/v1beta1:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = [
        "errors_test.go",
        "example_test.go",
        "stack_test.go",
    ],
    embed = [":go_default_library"],
    deps = [
        "//vendor/cloud.google.com/go/internal/testutil:go_default_library",
        "//vendor/golang.org/x/net/context:go_default_library",
        "//vendor/google.golang.org/api/option:go_default_library",
        "@com_github_googleapis_gax_go//:go_default_library",
        "@org_golang_google_genproto//googleapis/devtools/clouderrorreporting/v1beta1:go_default_library",
    ],
)
