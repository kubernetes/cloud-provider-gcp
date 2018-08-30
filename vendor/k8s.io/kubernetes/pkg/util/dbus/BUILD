package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
    "go_test",
)

go_library(
    name = "go_default_library",
    srcs = [
        "dbus.go",
        "doc.go",
        "fake_dbus.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/pkg/util/dbus",
    importpath = "k8s.io/kubernetes/pkg/util/dbus",
    deps = ["@com_github_godbus_dbus//:go_default_library"],
)

go_test(
    name = "go_default_test",
    srcs = ["dbus_test.go"],
    embed = [":go_default_library"],
    deps = ["@com_github_godbus_dbus//:go_default_library"],
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
