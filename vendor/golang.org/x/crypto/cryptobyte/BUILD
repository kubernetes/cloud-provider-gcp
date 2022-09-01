load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "cryptobyte",
    srcs = [
        "asn1.go",
        "builder.go",
        "string.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/golang.org/x/crypto/cryptobyte",
    importpath = "golang.org/x/crypto/cryptobyte",
    visibility = ["//visibility:public"],
    deps = ["//vendor/golang.org/x/crypto/cryptobyte/asn1"],
)
