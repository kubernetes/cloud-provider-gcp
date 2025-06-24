load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "namer",
    srcs = [
        "doc.go",
        "import_tracker.go",
        "namer.go",
        "order.go",
        "plural_namer.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/gengo/v2/namer",
    importpath = "k8s.io/gengo/v2/namer",
    visibility = ["//visibility:public"],
    deps = ["//vendor/k8s.io/gengo/v2/types"],
)
