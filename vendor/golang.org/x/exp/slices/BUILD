load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "slices",
    srcs = [
        "slices.go",
        "sort.go",
        "zsortfunc.go",
        "zsortordered.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/golang.org/x/exp/slices",
    importpath = "golang.org/x/exp/slices",
    visibility = ["//visibility:public"],
    deps = ["//vendor/golang.org/x/exp/constraints"],
)
