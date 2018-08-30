package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_library",
)

go_library(
    name = "go_default_library",
    srcs = ["etcd.go"],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/staging/src/k8s.io/kube-aggregator/pkg/registry/apiservice/etcd",
    importpath = "k8s.io/kubernetes/staging/src/k8s.io/kube-aggregator/pkg/registry/apiservice/etcd",
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/runtime:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/registry/generic:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/registry/generic/registry:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/registry/rest:go_default_library",
        "@io_k8s_kube_aggregator//pkg/apis/apiregistration:go_default_library",
        "@io_k8s_kube_aggregator//pkg/registry/apiservice:go_default_library",
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
