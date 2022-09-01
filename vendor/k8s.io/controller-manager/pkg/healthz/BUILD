load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "healthz",
    srcs = [
        "handler.go",
        "healthz.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/controller-manager/pkg/healthz",
    importpath = "k8s.io/controller-manager/pkg/healthz",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apiserver/pkg/server/healthz",
        "//vendor/k8s.io/apiserver/pkg/server/mux",
    ],
)
