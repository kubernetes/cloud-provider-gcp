load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "leaderelection",
    srcs = [
        "healthzadaptor.go",
        "leaderelection.go",
        "leasecandidate.go",
        "metrics.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/client-go/tools/leaderelection",
    importpath = "k8s.io/client-go/tools/leaderelection",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/coordination/v1:coordination",
        "//vendor/k8s.io/api/coordination/v1alpha1",
        "//vendor/k8s.io/apimachinery/pkg/api/errors",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/fields",
        "//vendor/k8s.io/apimachinery/pkg/util/runtime",
        "//vendor/k8s.io/apimachinery/pkg/util/wait",
        "//vendor/k8s.io/client-go/informers",
        "//vendor/k8s.io/client-go/kubernetes",
        "//vendor/k8s.io/client-go/kubernetes/typed/coordination/v1alpha1",
        "//vendor/k8s.io/client-go/tools/cache",
        "//vendor/k8s.io/client-go/tools/leaderelection/resourcelock",
        "//vendor/k8s.io/client-go/util/workqueue",
        "//vendor/k8s.io/klog/v2:klog",
        "//vendor/k8s.io/utils/clock",
    ],
)
