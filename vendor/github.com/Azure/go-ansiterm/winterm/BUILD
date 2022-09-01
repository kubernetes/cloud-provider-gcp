load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "winterm",
    srcs = [
        "ansi.go",
        "api.go",
        "attr_translation.go",
        "cursor_helpers.go",
        "erase_helpers.go",
        "scroll_helper.go",
        "utilities.go",
        "win_event_handler.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/Azure/go-ansiterm/winterm",
    importpath = "github.com/Azure/go-ansiterm/winterm",
    visibility = ["//visibility:public"],
    deps = select({
        "@io_bazel_rules_go//go/platform:windows": [
            "//vendor/github.com/Azure/go-ansiterm",
            "//vendor/golang.org/x/sys/windows",
        ],
        "//conditions:default": [],
    }),
)
