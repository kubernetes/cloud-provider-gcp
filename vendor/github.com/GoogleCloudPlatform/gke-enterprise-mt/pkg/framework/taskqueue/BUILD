load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "taskqueue",
    srcs = ["taskqueue.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/framework/taskqueue",
    importpath = "github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/framework/taskqueue",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/client-go/tools/cache",
        "//vendor/k8s.io/client-go/util/workqueue",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
