package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
)

go_library(
    name = "go_default_library",
    srcs = ["datafile.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/staging/src/k8s.io/apiserver/pkg/server/routes/data/swagger",
    importpath = "k8s.io/kubernetes/staging/src/k8s.io/apiserver/pkg/server/routes/data/swagger",
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
