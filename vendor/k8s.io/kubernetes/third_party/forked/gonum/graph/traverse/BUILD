package(default_visibility = ["//visibility:public"])

licenses(["notice"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
)

go_library(
    name = "go_default_library",
    srcs = [
        "traverse.go",
        "visit_depth_first.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/third_party/forked/gonum/graph/traverse",
    importpath = "k8s.io/kubernetes/third_party/forked/gonum/graph/traverse",
    deps = [
        "//vendor/k8s.io/kubernetes/third_party/forked/gonum/graph:go_default_library",
        "//vendor/k8s.io/kubernetes/third_party/forked/gonum/graph/internal/linear:go_default_library",
        "@org_golang_x_tools//container/intsets:go_default_library",
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
