load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "cert",
    srcs = [
        "default_cert.go",
        "enterprise_cert.go",
        "secureconnect_cert.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/google.golang.org/api/internal/cert",
    importpath = "google.golang.org/api/internal/cert",
    visibility = ["//vendor/google.golang.org/api:__subpackages__"],
    deps = ["//vendor/github.com/googleapis/enterprise-certificate-proxy/client"],
)
