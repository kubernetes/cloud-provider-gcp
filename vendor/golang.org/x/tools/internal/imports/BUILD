load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "imports",
    srcs = [
        "fix.go",
        "imports.go",
        "mod.go",
        "mod_cache.go",
        "sortimports.go",
        "source.go",
        "source_env.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/golang.org/x/tools/internal/imports",
    importpath = "golang.org/x/tools/internal/imports",
    visibility = ["//vendor/golang.org/x/tools:__subpackages__"],
    deps = [
        "//vendor/golang.org/x/mod/module",
        "//vendor/golang.org/x/sync/errgroup",
        "//vendor/golang.org/x/tools/go/ast/astutil",
        "//vendor/golang.org/x/tools/internal/event",
        "//vendor/golang.org/x/tools/internal/gocommand",
        "//vendor/golang.org/x/tools/internal/gopathwalk",
        "//vendor/golang.org/x/tools/internal/stdlib",
    ],
)
