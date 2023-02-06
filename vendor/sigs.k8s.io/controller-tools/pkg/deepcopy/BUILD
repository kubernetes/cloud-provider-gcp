load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "deepcopy",
    srcs = [
        "doc.go",
        "gen.go",
        "traverse.go",
        "zz_generated.markerhelp.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/sigs.k8s.io/controller-tools/pkg/deepcopy",
    importpath = "sigs.k8s.io/controller-tools/pkg/deepcopy",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/sigs.k8s.io/controller-tools/pkg/genall",
        "//vendor/sigs.k8s.io/controller-tools/pkg/loader",
        "//vendor/sigs.k8s.io/controller-tools/pkg/markers",
    ],
)
