load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "genall",
    srcs = [
        "doc.go",
        "genall.go",
        "input.go",
        "options.go",
        "output.go",
        "zz_generated.markerhelp.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/sigs.k8s.io/controller-tools/pkg/genall",
    importpath = "sigs.k8s.io/controller-tools/pkg/genall",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/golang.org/x/tools/go/packages",
        "//vendor/sigs.k8s.io/controller-tools/pkg/loader",
        "//vendor/sigs.k8s.io/controller-tools/pkg/markers",
        "//vendor/sigs.k8s.io/yaml",
    ],
)
