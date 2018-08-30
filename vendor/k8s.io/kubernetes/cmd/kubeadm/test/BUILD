package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
)

go_library(
    name = "go_default_library",
    srcs = ["util.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/cmd/kubeadm/test",
    importpath = "k8s.io/kubernetes/cmd/kubeadm/test",
    deps = [
        "//vendor/k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm:go_default_library",
        "//vendor/k8s.io/kubernetes/cmd/kubeadm/app/constants:go_default_library",
        "//vendor/k8s.io/kubernetes/cmd/kubeadm/app/phases/certs/pkiutil:go_default_library",
        "//vendor/k8s.io/kubernetes/cmd/kubeadm/test/certs:go_default_library",
        "@com_github_renstrom_dedent//:go_default_library",
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
        "//cmd/kubeadm/test/certs:all-srcs",
        "//cmd/kubeadm/test/cmd:all-srcs",
        "//cmd/kubeadm/test/kubeconfig:all-srcs",
    ],
    tags = ["automanaged"],
)
