load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "ports",
    srcs = [
        "doc.go",
        "ports.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/kubernetes/pkg/cluster/ports",
    importpath = "k8s.io/kubernetes/pkg/cluster/ports",
    visibility = ["//visibility:public"],
    deps = ["//vendor/k8s.io/cloud-provider/options"],
)
