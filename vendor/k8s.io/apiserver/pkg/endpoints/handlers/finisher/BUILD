load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "finisher",
    srcs = ["finisher.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/endpoints/handlers/finisher",
    importpath = "k8s.io/apiserver/pkg/endpoints/handlers/finisher",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/api/errors",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/util/runtime",
        "//vendor/k8s.io/apiserver/pkg/endpoints/metrics",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
