load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "impersonate",
    srcs = ["impersonate.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/golang.org/x/oauth2/google/internal/impersonate",
    importpath = "golang.org/x/oauth2/google/internal/impersonate",
    visibility = ["//vendor/golang.org/x/oauth2/google:__subpackages__"],
    deps = ["//vendor/golang.org/x/oauth2"],
)
