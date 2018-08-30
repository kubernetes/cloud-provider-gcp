load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["main.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/test/integration/benchmark/jsonify",
    importpath = "k8s.io/kubernetes/test/integration/benchmark/jsonify",
    visibility = ["//visibility:private"],
    deps = [
        "//vendor/k8s.io/kubernetes/test/e2e/perftype:go_default_library",
        "@org_golang_x_tools//benchmark/parse:go_default_library",
    ],
)

go_binary(
    name = "jsonify",
    embed = [":go_default_library"],
    visibility = ["//visibility:public"],
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
    visibility = ["//visibility:public"],
)
