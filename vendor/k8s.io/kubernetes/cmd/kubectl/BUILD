load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_binary",
    "go_library",
)
load("//pkg/version:def.bzl", "version_x_defs")

go_binary(
    name = "kubectl",
    embed = [":go_default_library"],
    pure = "on",
    visibility = ["//visibility:public"],
    x_defs = version_x_defs(),
)

go_library(
    name = "go_default_library",
    srcs = ["kubectl.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/cmd/kubectl",
    importpath = "k8s.io/kubernetes/cmd/kubectl",
    visibility = ["//visibility:private"],
    deps = [
        "//vendor/github.com/spf13/pflag:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/util/flag:go_default_library",
        "//vendor/k8s.io/client-go/plugin/pkg/client/auth:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubectl/cmd:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/kubectl/util/logs:go_default_library",
    ],
)

filegroup(
    name = "package-srcs",
    srcs = glob(["**"]),
    tags = ["automanaged"],
)

filegroup(
    name = "all-srcs",
    srcs = [":package-srcs"],
    tags = ["automanaged"],
    visibility = [
        "//build/visible_to:cmd_kubectl_CONSUMERS",
    ],
)
