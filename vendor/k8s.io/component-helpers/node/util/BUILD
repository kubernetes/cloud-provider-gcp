load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "util",
    srcs = [
        "cidr.go",
        "conditions.go",
        "hostname.go",
        "ips.go",
        "status.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/component-helpers/node/util",
    importpath = "k8s.io/component-helpers/node/util",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/core/v1:core",
        "//vendor/k8s.io/apimachinery/pkg/api/equality",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/types",
        "//vendor/k8s.io/apimachinery/pkg/util/strategicpatch",
        "//vendor/k8s.io/client-go/kubernetes",
        "//vendor/k8s.io/client-go/kubernetes/typed/core/v1:core",
        "//vendor/k8s.io/klog/v2:klog",
        "//vendor/k8s.io/utils/net",
    ],
)
