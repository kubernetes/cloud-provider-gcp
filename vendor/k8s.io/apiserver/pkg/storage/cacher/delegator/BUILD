load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "delegator",
    srcs = ["interface.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/storage/cacher/delegator",
    importpath = "k8s.io/apiserver/pkg/storage/cacher/delegator",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apiserver/pkg/features",
        "//vendor/k8s.io/apiserver/pkg/storage",
        "//vendor/k8s.io/apiserver/pkg/storage/feature",
        "//vendor/k8s.io/apiserver/pkg/util/feature",
    ],
)
