load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "impersonate",
    srcs = [
        "doc.go",
        "idtoken.go",
        "impersonate.go",
        "user.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/google.golang.org/api/impersonate",
    importpath = "google.golang.org/api/impersonate",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/golang.org/x/oauth2",
        "//vendor/google.golang.org/api/option",
        "//vendor/google.golang.org/api/option/internaloption",
        "//vendor/google.golang.org/api/transport/http",
    ],
)
