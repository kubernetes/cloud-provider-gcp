load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "buffered",
    srcs = [
        "buffered.go",
        "doc.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/plugin/pkg/audit/buffered",
    importpath = "k8s.io/apiserver/plugin/pkg/audit/buffered",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/util/runtime",
        "//vendor/k8s.io/apimachinery/pkg/util/wait",
        "//vendor/k8s.io/apiserver/pkg/apis/audit",
        "//vendor/k8s.io/apiserver/pkg/audit",
        "//vendor/k8s.io/client-go/util/flowcontrol",
    ],
)
