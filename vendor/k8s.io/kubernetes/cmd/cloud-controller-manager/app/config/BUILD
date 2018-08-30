load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["config.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/cmd/cloud-controller-manager/app/config",
    importpath = "k8s.io/kubernetes/cmd/cloud-controller-manager/app/config",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apiserver/pkg/server:go_default_library",
        "//vendor/k8s.io/client-go/kubernetes:go_default_library",
        "//vendor/k8s.io/client-go/rest:go_default_library",
        "//vendor/k8s.io/client-go/tools/record:go_default_library",
        "//vendor/k8s.io/kubernetes/cmd/controller-manager/app:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/apis/componentconfig:go_default_library",
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
    visibility = ["//visibility:public"],
)
