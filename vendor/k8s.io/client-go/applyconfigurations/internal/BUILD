load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "internal",
    srcs = ["internal.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/client-go/applyconfigurations/internal",
    importpath = "k8s.io/client-go/applyconfigurations/internal",
    visibility = ["//vendor/k8s.io/client-go/applyconfigurations:__subpackages__"],
    deps = ["//vendor/sigs.k8s.io/structured-merge-diff/v4/typed"],
)
