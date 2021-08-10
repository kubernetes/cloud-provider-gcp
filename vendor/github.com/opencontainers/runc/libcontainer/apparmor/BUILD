load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "apparmor.go",
        "apparmor_linux.go",
        "apparmor_unsupported.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/opencontainers/runc/libcontainer/apparmor",
    importpath = "github.com/opencontainers/runc/libcontainer/apparmor",
    visibility = ["//visibility:public"],
    deps = select({
        "@io_bazel_rules_go//go/platform:android": [
            "//vendor/github.com/opencontainers/runc/libcontainer/utils:go_default_library",
        ],
        "@io_bazel_rules_go//go/platform:linux": [
            "//vendor/github.com/opencontainers/runc/libcontainer/utils:go_default_library",
        ],
        "//conditions:default": [],
    }),
)
