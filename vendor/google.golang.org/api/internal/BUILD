load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "creds.go",
        "pool.go",
        "settings.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/google.golang.org/api/internal",
    importpath = "google.golang.org/api/internal",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/golang.org/x/net/context:go_default_library",
        "//vendor/golang.org/x/oauth2:go_default_library",
        "//vendor/golang.org/x/oauth2/google:go_default_library",
        "@org_golang_google_grpc//:go_default_library",
        "@org_golang_google_grpc//naming:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = [
        "creds_test.go",
        "pool_test.go",
        "settings_test.go",
    ],
    embed = [":go_default_library"],
    deps = [
        "//vendor/golang.org/x/net/context:go_default_library",
        "//vendor/golang.org/x/oauth2:go_default_library",
        "//vendor/golang.org/x/oauth2/google:go_default_library",
        "@com_github_google_go_cmp//cmp:go_default_library",
        "@org_golang_google_grpc//:go_default_library",
        "@org_golang_google_grpc//naming:go_default_library",
    ],
)
