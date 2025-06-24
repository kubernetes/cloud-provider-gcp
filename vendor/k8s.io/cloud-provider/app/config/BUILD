load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "config",
    srcs = ["config.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/cloud-provider/app/config",
    importpath = "k8s.io/cloud-provider/app/config",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apiserver/pkg/server",
        "//vendor/k8s.io/client-go/informers",
        "//vendor/k8s.io/client-go/kubernetes",
        "//vendor/k8s.io/client-go/rest",
        "//vendor/k8s.io/client-go/tools/record",
        "//vendor/k8s.io/cloud-provider",
        "//vendor/k8s.io/cloud-provider/config",
    ],
)
