package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
)

go_library(
    name = "go_default_library",
    srcs = [
        "resource_container_linux.go",
        "resource_container_unsupported.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/pkg/util/resourcecontainer",
    importpath = "k8s.io/kubernetes/pkg/util/resourcecontainer",
    deps = select({
        "@io_bazel_rules_go//go/platform:linux": [
            "@com_github_opencontainers_runc//libcontainer/cgroups/fs:go_default_library",
            "@com_github_opencontainers_runc//libcontainer/configs:go_default_library",
        ],
        "//conditions:default": [],
    }),
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
