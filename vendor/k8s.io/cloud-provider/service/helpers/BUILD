load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "helpers",
    srcs = ["helper.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/cloud-provider/service/helpers",
    importpath = "k8s.io/cloud-provider/service/helpers",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/core/v1:core",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/types",
        "//vendor/k8s.io/apimachinery/pkg/util/strategicpatch",
        "//vendor/k8s.io/client-go/kubernetes/typed/core/v1:core",
        "//vendor/k8s.io/utils/net",
    ],
)
