load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "nodelifecycle",
    srcs = ["node_lifecycle_controller.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/cloud-provider/controllers/nodelifecycle",
    importpath = "k8s.io/cloud-provider/controllers/nodelifecycle",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/core/v1:core",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/labels",
        "//vendor/k8s.io/apimachinery/pkg/types",
        "//vendor/k8s.io/apimachinery/pkg/util/runtime",
        "//vendor/k8s.io/apimachinery/pkg/util/wait",
        "//vendor/k8s.io/client-go/informers/core/v1:core",
        "//vendor/k8s.io/client-go/kubernetes",
        "//vendor/k8s.io/client-go/kubernetes/scheme",
        "//vendor/k8s.io/client-go/kubernetes/typed/core/v1:core",
        "//vendor/k8s.io/client-go/listers/core/v1:core",
        "//vendor/k8s.io/client-go/tools/record",
        "//vendor/k8s.io/cloud-provider",
        "//vendor/k8s.io/cloud-provider/api",
        "//vendor/k8s.io/cloud-provider/node/helpers",
        "//vendor/k8s.io/component-base/metrics/prometheus/controllers",
        "//vendor/k8s.io/component-helpers/node/util",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
