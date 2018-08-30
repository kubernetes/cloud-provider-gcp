package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
)

go_library(
    name = "go_default_library",
    srcs = ["generator_for_scheme.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/staging/src/k8s.io/code-generator/cmd/client-gen/generators/scheme",
    importpath = "k8s.io/kubernetes/staging/src/k8s.io/code-generator/cmd/client-gen/generators/scheme",
    deps = [
        "@io_k8s_code_generator//cmd/client-gen/path:go_default_library",
        "@io_k8s_code_generator//cmd/client-gen/types:go_default_library",
        "@io_k8s_gengo//generator:go_default_library",
        "@io_k8s_gengo//namer:go_default_library",
        "@io_k8s_gengo//types:go_default_library",
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
