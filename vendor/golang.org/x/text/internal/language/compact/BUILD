load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "compact",
    srcs = [
        "compact.go",
        "language.go",
        "parents.go",
        "tables.go",
        "tags.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/golang.org/x/text/internal/language/compact",
    importpath = "golang.org/x/text/internal/language/compact",
    visibility = ["//vendor/golang.org/x/text:__subpackages__"],
    deps = ["//vendor/golang.org/x/text/internal/language"],
)
