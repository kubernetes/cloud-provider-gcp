package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
)

go_library(
    name = "go_default_library",
    srcs = ["latest.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/pkg/apis/abac/latest",
    importpath = "k8s.io/kubernetes/pkg/apis/abac/latest",
    deps = [
        "//vendor/k8s.io/kubernetes/pkg/apis/abac:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/apis/abac/v0:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/apis/abac/v1beta1:go_default_library",
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
