load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "credentials_go19.go",
        "credentials_notgo19.go",
        "option.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/google.golang.org/api/option",
    importpath = "google.golang.org/api/option",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/golang.org/x/oauth2:go_default_library",
        "//vendor/golang.org/x/oauth2/google:go_default_library",
        "//vendor/google.golang.org/api/internal:go_default_library",
        "@org_golang_google_grpc//:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = ["option_test.go"],
    embed = [":go_default_library"],
    deps = [
        "//vendor/golang.org/x/oauth2/google:go_default_library",
        "//vendor/google.golang.org/api/internal:go_default_library",
        "@com_github_google_go_cmp//cmp:go_default_library",
        "@com_github_google_go_cmp//cmp/cmpopts:go_default_library",
        "@org_golang_google_grpc//:go_default_library",
    ],
)
