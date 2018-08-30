load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["etcd.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/cmd/kubeadm/app/util/etcd",
    importpath = "k8s.io/kubernetes/cmd/kubeadm/app/util/etcd",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm:go_default_library",
        "//vendor/k8s.io/kubernetes/cmd/kubeadm/app/constants:go_default_library",
        "//vendor/k8s.io/kubernetes/cmd/kubeadm/app/util/staticpod:go_default_library",
        "@com_github_coreos_etcd//clientv3:go_default_library",
        "@com_github_coreos_etcd//pkg/transport:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = ["etcd_test.go"],
    embed = [":go_default_library"],
    deps = [
        "//vendor/k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm:go_default_library",
        "//vendor/k8s.io/kubernetes/cmd/kubeadm/test:go_default_library",
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
