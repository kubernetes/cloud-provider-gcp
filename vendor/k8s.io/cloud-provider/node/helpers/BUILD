load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "helpers",
    srcs = [
        "address.go",
        "labels.go",
        "taints.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/cloud-provider/node/helpers",
    importpath = "k8s.io/cloud-provider/node/helpers",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/core/v1:core",
        "//vendor/k8s.io/apimachinery/pkg/api/equality",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/types",
        "//vendor/k8s.io/apimachinery/pkg/util/runtime",
        "//vendor/k8s.io/apimachinery/pkg/util/strategicpatch",
        "//vendor/k8s.io/apimachinery/pkg/util/wait",
        "//vendor/k8s.io/client-go/kubernetes",
        "//vendor/k8s.io/client-go/util/retry",
        "//vendor/k8s.io/component-helpers/node/util",
        "//vendor/k8s.io/klog/v2:klog",
        "//vendor/k8s.io/utils/net",
    ],
)
