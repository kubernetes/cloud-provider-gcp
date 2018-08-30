package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
)

go_library(
    name = "go_default_library",
    srcs = [
        "cgroups_linux.go",
        "cgroups_unsupported.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/pkg/kubelet/cm/util",
    importpath = "k8s.io/kubernetes/pkg/kubelet/cm/util",
    deps = select({
        "@io_bazel_rules_go//go/platform:linux": [
            "@com_github_opencontainers_runc//libcontainer/cgroups:go_default_library",
            "@com_github_opencontainers_runc//libcontainer/utils:go_default_library",
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
