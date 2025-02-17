load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "testrestmapper",
    srcs = ["test_restmapper.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apimachinery/pkg/api/meta/testrestmapper",
    importpath = "k8s.io/apimachinery/pkg/api/meta/testrestmapper",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/api/meta",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
        "//vendor/k8s.io/apimachinery/pkg/util/sets",
    ],
)
