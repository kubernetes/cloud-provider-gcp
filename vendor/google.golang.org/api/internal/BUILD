load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "internal",
    srcs = [
        "conn_pool.go",
        "creds.go",
        "dca.go",
        "settings.go",
        "version.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/google.golang.org/api/internal",
    importpath = "google.golang.org/api/internal",
    visibility = ["//vendor/google.golang.org/api:__subpackages__"],
    deps = [
        "//vendor/golang.org/x/oauth2",
        "//vendor/golang.org/x/oauth2/google",
        "//vendor/google.golang.org/api/internal/cert",
        "//vendor/google.golang.org/api/internal/impersonate",
        "//vendor/google.golang.org/grpc",
    ],
)
