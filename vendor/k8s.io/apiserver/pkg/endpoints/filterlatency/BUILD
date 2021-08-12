load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["filterlatency.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/endpoints/filterlatency",
    importpath = "k8s.io/apiserver/pkg/endpoints/filterlatency",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/util/clock:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/endpoints/metrics:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/endpoints/request:go_default_library",
    ],
)
