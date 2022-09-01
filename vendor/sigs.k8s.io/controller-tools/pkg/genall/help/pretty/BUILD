load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "pretty",
    srcs = [
        "doc.go",
        "help.go",
        "print.go",
        "table.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/sigs.k8s.io/controller-tools/pkg/genall/help/pretty",
    importpath = "sigs.k8s.io/controller-tools/pkg/genall/help/pretty",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/fatih/color",
        "//vendor/sigs.k8s.io/controller-tools/pkg/genall/help",
    ],
)
