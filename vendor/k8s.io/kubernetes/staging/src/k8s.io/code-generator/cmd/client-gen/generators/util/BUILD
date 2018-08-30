package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
    "go_test",
)

go_test(
    name = "go_default_test",
    srcs = ["tags_test.go"],
    embed = [":go_default_library"],
)

go_library(
    name = "go_default_library",
    srcs = ["tags.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/staging/src/k8s.io/code-generator/cmd/client-gen/generators/util",
    importpath = "k8s.io/kubernetes/staging/src/k8s.io/code-generator/cmd/client-gen/generators/util",
    deps = ["@io_k8s_gengo//types:go_default_library"],
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
