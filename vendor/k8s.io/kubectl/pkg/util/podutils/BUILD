load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "podutils",
    srcs = ["podutils.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/kubectl/pkg/util/podutils",
    importpath = "k8s.io/kubectl/pkg/util/podutils",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/core/v1:core",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/util/sets",
    ],
)
