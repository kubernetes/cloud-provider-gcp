load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "fairqueuing",
    srcs = [
        "integrator.go",
        "interface.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/util/flowcontrol/fairqueuing",
    importpath = "k8s.io/apiserver/pkg/util/flowcontrol/fairqueuing",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apiserver/pkg/util/flowcontrol/debug",
        "//vendor/k8s.io/apiserver/pkg/util/flowcontrol/metrics",
        "//vendor/k8s.io/apiserver/pkg/util/flowcontrol/request",
        "//vendor/k8s.io/utils/clock",
    ],
)
