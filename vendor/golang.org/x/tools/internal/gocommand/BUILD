load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "gocommand",
    srcs = [
        "invoke.go",
        "vendor.go",
        "version.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/golang.org/x/tools/internal/gocommand",
    importpath = "golang.org/x/tools/internal/gocommand",
    visibility = ["//vendor/golang.org/x/tools:__subpackages__"],
    deps = [
        "//vendor/golang.org/x/mod/semver",
        "//vendor/golang.org/x/tools/internal/event",
        "//vendor/golang.org/x/tools/internal/event/keys",
        "//vendor/golang.org/x/tools/internal/event/label",
        "//vendor/golang.org/x/tools/internal/event/tag",
    ],
)
