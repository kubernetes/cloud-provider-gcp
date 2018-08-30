package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
)

go_library(
    name = "go_default_library",
    srcs = [
        "defaultfs.go",
        "fakefs.go",
        "filesystem.go",
        "watcher.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/pkg/util/filesystem",
    importpath = "k8s.io/kubernetes/pkg/util/filesystem",
    deps = [
        "@com_github_fsnotify_fsnotify//:go_default_library",
        "@com_github_spf13_afero//:go_default_library",
    ],
)

filegroup(
    name = "package-srcs",
    srcs = glob(["**"]),
    tags = ["automanaged"],
    visibility = ["//visibility:private"],
)

filegroup(
    name = "all-srcs",
    srcs = [":package-srcs"],
    tags = ["automanaged"],
)
