load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "x509metrics",
    srcs = ["server_cert_deprecations.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/util/x509metrics",
    importpath = "k8s.io/apiserver/pkg/util/x509metrics",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/util/net",
        "//vendor/k8s.io/apiserver/pkg/audit",
        "//vendor/k8s.io/component-base/metrics",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
