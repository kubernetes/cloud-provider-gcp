load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "handler.go",
        "healthz.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/controller-manager/pkg/healthz",
    importpath = "k8s.io/controller-manager/pkg/healthz",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apiserver/pkg/server/healthz:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/server/mux:go_default_library",
    ],
)
