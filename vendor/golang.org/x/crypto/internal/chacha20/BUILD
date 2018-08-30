load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "asm_s390x.s",
        "chacha_generic.go",
        "chacha_noasm.go",
        "chacha_s390x.go",
        "xor.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/golang.org/x/crypto/internal/chacha20",
    importpath = "golang.org/x/crypto/internal/chacha20",
    visibility = ["//vendor/golang.org/x/crypto:__subpackages__"],
    deps = ["//vendor/golang.org/x/crypto/internal/subtle:go_default_library"],
)

go_test(
    name = "go_default_test",
    srcs = [
        "chacha_test.go",
        "vectors_test.go",
    ],
    embed = [":go_default_library"],
)
