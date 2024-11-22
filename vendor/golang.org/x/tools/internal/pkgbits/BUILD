load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "pkgbits",
    srcs = [
        "codes.go",
        "decoder.go",
        "doc.go",
        "encoder.go",
        "flags.go",
        "reloc.go",
        "support.go",
        "sync.go",
        "syncmarker_string.go",
        "version.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/golang.org/x/tools/internal/pkgbits",
    importpath = "golang.org/x/tools/internal/pkgbits",
    visibility = ["//vendor/golang.org/x/tools:__subpackages__"],
)
