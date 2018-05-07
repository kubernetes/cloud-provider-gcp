load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "cert.go",
        "csr.go",
        "io.go",
        "pem.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/client-go/util/cert",
    importpath = "k8s.io/client-go/util/cert",
    visibility = ["//visibility:public"],
)
