load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "generator",
    srcs = [
        "doc.go",
        "error_tracker.go",
        "execute.go",
        "generator.go",
        "go_generator.go",
        "import_tracker.go",
        "simple_target.go",
        "snippet_writer.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/gengo/v2/generator",
    importpath = "k8s.io/gengo/v2/generator",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/golang.org/x/tools/imports",
        "//vendor/k8s.io/gengo/v2/namer",
        "//vendor/k8s.io/gengo/v2/parser",
        "//vendor/k8s.io/gengo/v2/types",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
