package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_binary",
    "go_library",
)

go_library(
    name = "go_default_library",
    srcs = ["serve_hostname.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/test/images/serve-hostname",
    importpath = "k8s.io/kubernetes/test/images/serve-hostname",
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

go_binary(
    name = "serve-hostname",
    embed = [":go_default_library"],
)
