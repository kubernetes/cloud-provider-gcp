load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "certificates",
    srcs = [
        "certificate_controller.go",
        "certificate_controller_utils.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/kubernetes/pkg/controller/certificates",
    importpath = "k8s.io/kubernetes/pkg/controller/certificates",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/golang.org/x/time/rate",
        "//vendor/k8s.io/api/certificates/v1:certificates",
        "//vendor/k8s.io/api/core/v1:core",
        "//vendor/k8s.io/apimachinery/pkg/api/errors",
        "//vendor/k8s.io/apimachinery/pkg/util/runtime",
        "//vendor/k8s.io/apimachinery/pkg/util/wait",
        "//vendor/k8s.io/client-go/informers/certificates/v1:certificates",
        "//vendor/k8s.io/client-go/kubernetes",
        "//vendor/k8s.io/client-go/listers/certificates/v1:certificates",
        "//vendor/k8s.io/client-go/tools/cache",
        "//vendor/k8s.io/client-go/util/workqueue",
        "//vendor/k8s.io/klog/v2:klog",
        "//vendor/k8s.io/kubernetes/pkg/controller",
    ],
)
