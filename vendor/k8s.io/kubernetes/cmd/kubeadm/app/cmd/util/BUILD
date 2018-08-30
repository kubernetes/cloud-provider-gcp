load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "cmdutil.go",
        "documentation.go",
        "join.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/cmd/kubeadm/app/cmd/util",
    importpath = "k8s.io/kubernetes/cmd/kubeadm/app/cmd/util",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/spf13/cobra:go_default_library",
        "//vendor/k8s.io/client-go/tools/clientcmd:go_default_library",
        "//vendor/k8s.io/client-go/util/cert:go_default_library",
        "//vendor/k8s.io/kubernetes/cmd/kubeadm/app/util/kubeconfig:go_default_library",
        "//vendor/k8s.io/kubernetes/cmd/kubeadm/app/util/pubkeypin:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/util/normalizer:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = ["cmdutil_test.go"],
    embed = [":go_default_library"],
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
