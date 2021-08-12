load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "integrator.go",
        "interface.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/util/flowcontrol/fairqueuing",
    importpath = "k8s.io/apiserver/pkg/util/flowcontrol/fairqueuing",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/util/clock:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/util/flowcontrol/debug:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/util/flowcontrol/metrics:go_default_library",
    ],
)
