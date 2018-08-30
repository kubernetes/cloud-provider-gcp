package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
)

go_library(
    name = "go_default_library",
    srcs = ["storage.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/pkg/registry/rbac/clusterrolebinding/storage",
    importpath = "k8s.io/kubernetes/pkg/registry/rbac/clusterrolebinding/storage",
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/runtime:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/registry/generic:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/registry/generic/registry:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/apis/rbac:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/printers:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/printers/internalversion:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/printers/storage:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/registry/rbac/clusterrolebinding:go_default_library",
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
