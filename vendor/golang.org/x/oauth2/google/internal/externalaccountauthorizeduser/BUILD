load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "externalaccountauthorizeduser",
    srcs = ["externalaccountauthorizeduser.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/golang.org/x/oauth2/google/internal/externalaccountauthorizeduser",
    importpath = "golang.org/x/oauth2/google/internal/externalaccountauthorizeduser",
    visibility = ["//vendor/golang.org/x/oauth2/google:__subpackages__"],
    deps = [
        "//vendor/golang.org/x/oauth2",
        "//vendor/golang.org/x/oauth2/google/internal/stsexchange",
    ],
)
