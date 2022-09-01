load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "install",
    srcs = ["install.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/apis/audit/install",
    importpath = "k8s.io/apiserver/pkg/apis/audit/install",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/util/runtime",
        "//vendor/k8s.io/apiserver/pkg/apis/audit",
        "//vendor/k8s.io/apiserver/pkg/apis/audit/v1:audit",
    ],
)
