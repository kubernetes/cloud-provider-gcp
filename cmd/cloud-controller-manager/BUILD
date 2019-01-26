load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["controller-manager.go"],
    importpath = "k8s.io/cloud-provider-gcp/cmd/cloud-controller-manager",
    deps = [
        "//cmd/cloud-controller-manager/app/cloudprovider:go_default_library",
        "//vendor/k8s.io/component-base/logs:go_default_library",
        "//vendor/k8s.io/kubernetes/cmd/cloud-controller-manager/app:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/util/prometheusclientgo:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/version/prometheus:go_default_library",
    ],
)

go_binary(
    name = "cloud-controller-manager",
    embed = [":go_default_library"],
    pure = "on",
    visibility = ["//visibility:public"],
)

load("//defs:container.bzl", "image")

image(binary = ":cloud-controller-manager")
