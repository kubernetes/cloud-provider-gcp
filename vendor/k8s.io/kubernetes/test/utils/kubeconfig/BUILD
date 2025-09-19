load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "kubeconfig",
    srcs = ["kubeconfig.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/kubernetes/test/utils/kubeconfig",
    importpath = "k8s.io/kubernetes/test/utils/kubeconfig",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/client-go/rest",
        "//vendor/k8s.io/client-go/tools/clientcmd/api",
    ],
)
