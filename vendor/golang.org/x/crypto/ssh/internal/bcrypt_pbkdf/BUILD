load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "bcrypt_pbkdf",
    srcs = ["bcrypt_pbkdf.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/golang.org/x/crypto/ssh/internal/bcrypt_pbkdf",
    importpath = "golang.org/x/crypto/ssh/internal/bcrypt_pbkdf",
    visibility = ["//vendor/golang.org/x/crypto/ssh:__subpackages__"],
    deps = ["//vendor/golang.org/x/crypto/blowfish"],
)
