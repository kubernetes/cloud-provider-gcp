load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "modindex",
    srcs = [
        "directories.go",
        "index.go",
        "lookup.go",
        "modindex.go",
        "symbols.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/golang.org/x/tools/internal/modindex",
    importpath = "golang.org/x/tools/internal/modindex",
    visibility = ["//vendor/golang.org/x/tools:__subpackages__"],
    deps = [
        "//vendor/golang.org/x/mod/semver",
        "//vendor/golang.org/x/sync/errgroup",
        "//vendor/golang.org/x/tools/internal/gopathwalk",
    ],
)
