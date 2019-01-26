load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_binary",
    "go_library",
)

go_library(
    name = "go_default_library",
    srcs = ["main.go"],
    importpath = "k8s.io/cloud-provider-gcp/cmd/gcp-controller-manager",
    deps = [
        "//cmd/gcp-controller-manager/app:go_default_library",
        "//vendor/github.com/prometheus/client_golang/prometheus/promhttp:go_default_library",
        "//vendor/github.com/spf13/pflag:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubectl/util/logs:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/version/verflag:go_default_library",
    ],
)

go_binary(
    name = "gcp-controller-manager",
    embed = [":go_default_library"],
    pure = "on",
    visibility = ["//visibility:public"],
)

load("//defs:container.bzl", "image")

image(binary = ":gcp-controller-manager")
