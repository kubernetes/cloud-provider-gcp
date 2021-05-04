package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_binary",
    "go_library",
)
load("//defs:version.bzl", "version_x_defs")

go_binary(
    name = "cloud-controller-manager",
    embed = [":go_default_library"],
    pure = "on",
    x_defs = version_x_defs(),
)

go_library(
    name = "go_default_library",
    srcs = ["main.go"],
    importpath = "k8s.io/cloud-provider-gcp/cmd/cloud-controller-manager",
    deps = [
        "//providers/gce:go_default_library",
        "//vendor/github.com/spf13/pflag:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/wait:go_default_library",
        "//vendor/k8s.io/cloud-provider:go_default_library",
        "//vendor/k8s.io/cloud-provider/app:go_default_library",
        "//vendor/k8s.io/cloud-provider/app/config:go_default_library",
        "//vendor/k8s.io/cloud-provider/options:go_default_library",
        "//vendor/k8s.io/component-base/cli/flag:go_default_library",
        "//vendor/k8s.io/component-base/logs:go_default_library",
        "//vendor/k8s.io/component-base/metrics/prometheus/clientgo:go_default_library",
        "//vendor/k8s.io/component-base/metrics/prometheus/version:go_default_library",
        "//vendor/k8s.io/klog/v2:go_default_library",
    ],
)

load("//defs:container.bzl", "image")

image(binary = ":cloud-controller-manager")
