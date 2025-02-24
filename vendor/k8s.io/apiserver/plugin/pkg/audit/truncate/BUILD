load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "truncate",
    srcs = [
        "doc.go",
        "truncate.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/plugin/pkg/audit/truncate",
    importpath = "k8s.io/apiserver/plugin/pkg/audit/truncate",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
        "//vendor/k8s.io/apimachinery/pkg/util/errors",
        "//vendor/k8s.io/apiserver/pkg/apis/audit",
        "//vendor/k8s.io/apiserver/pkg/audit",
    ],
)
