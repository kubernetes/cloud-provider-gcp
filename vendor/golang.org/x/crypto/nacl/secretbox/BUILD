load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "secretbox",
    srcs = ["secretbox.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/golang.org/x/crypto/nacl/secretbox",
    importpath = "golang.org/x/crypto/nacl/secretbox",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/golang.org/x/crypto/internal/alias",
        "//vendor/golang.org/x/crypto/internal/poly1305",
        "//vendor/golang.org/x/crypto/salsa20/salsa",
    ],
)
