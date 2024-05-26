load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "flock",
    srcs = [
        "flock.go",
        "flock_aix.go",
        "flock_unix.go",
        "flock_winapi.go",
        "flock_windows.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/gofrs/flock",
    importpath = "github.com/gofrs/flock",
    visibility = ["//visibility:public"],
    deps = select({
        "@io_bazel_rules_go//go/platform:aix": [
            "//vendor/golang.org/x/sys/unix",
        ],
        "//conditions:default": [],
    }),
)
