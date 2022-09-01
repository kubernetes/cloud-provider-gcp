load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "windows",
    srcs = [
        "ansi_reader.go",
        "ansi_writer.go",
        "console.go",
        "doc.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/moby/term/windows",
    importpath = "github.com/moby/term/windows",
    visibility = ["//visibility:public"],
    deps = select({
        "@io_bazel_rules_go//go/platform:windows": [
            "//vendor/github.com/Azure/go-ansiterm",
            "//vendor/github.com/Azure/go-ansiterm/winterm",
            "//vendor/golang.org/x/sys/windows",
        ],
        "//conditions:default": [],
    }),
)
