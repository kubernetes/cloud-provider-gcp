package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
)

go_library(
    name = "go_default_library",
    srcs = [
        "cloud.go",
        "doc.go",
        "plugins.go",
    ],
    importmap = "k8s.io/kubernetes/vendor/k8s.io/cloud-provider",
    importpath = "k8s.io/cloud-provider",
    deps = [
        "//vendor/k8s.io/api/core/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/types:go_default_library",
        "//vendor/k8s.io/client-go/informers:go_default_library",
        "//vendor/k8s.io/client-go/kubernetes:go_default_library",
        "//vendor/k8s.io/client-go/rest:go_default_library",
        "//vendor/k8s.io/klog:go_default_library",
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
    srcs = [
        ":package-srcs",
        "//vendor/k8s.io/cloud-provider/fake:all-srcs",
        "//vendor/k8s.io/cloud-provider/node:all-srcs",
        "//vendor/k8s.io/cloud-provider/service/helpers:all-srcs",
        "//vendor/k8s.io/cloud-provider/volume:all-srcs",
    ],
    tags = ["automanaged"],
)
