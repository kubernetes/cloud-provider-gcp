load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "typesinternal",
    srcs = [
        "errorcode.go",
        "errorcode_string.go",
        "recv.go",
        "toonew.go",
        "types.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/golang.org/x/tools/internal/typesinternal",
    importpath = "golang.org/x/tools/internal/typesinternal",
    visibility = ["//vendor/golang.org/x/tools:__subpackages__"],
    deps = [
        "//vendor/golang.org/x/tools/internal/aliases",
        "//vendor/golang.org/x/tools/internal/stdlib",
        "//vendor/golang.org/x/tools/internal/versions",
    ],
)
