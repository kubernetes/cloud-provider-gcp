load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "textlogger",
    srcs = [
        "options.go",
        "textlogger.go",
        "textlogger_slog.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/klog/v2/textlogger",
    importpath = "k8s.io/klog/v2/textlogger",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/go-logr/logr",
        "//vendor/k8s.io/klog/v2/internal/buffer",
        "//vendor/k8s.io/klog/v2/internal/serialize",
        "//vendor/k8s.io/klog/v2/internal/severity",
        "//vendor/k8s.io/klog/v2/internal/sloghandler",
        "//vendor/k8s.io/klog/v2/internal/verbosity",
    ],
)
