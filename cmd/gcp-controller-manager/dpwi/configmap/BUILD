load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "configmap",
    srcs = ["configmap.go"],
    importpath = "k8s.io/cloud-provider-gcp/cmd/gcp-controller-manager/dpwi/configmap",
    visibility = ["//visibility:public"],
    deps = [
        "//cmd/gcp-controller-manager/dpwi/hms",
        "//cmd/gcp-controller-manager/dpwi/serviceaccounts",
        "//vendor/google.golang.org/api/compute/v1:compute",
        "//vendor/k8s.io/api/core/v1:core",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/util/wait",
        "//vendor/k8s.io/client-go/informers/core/v1:core",
        "//vendor/k8s.io/client-go/kubernetes",
        "//vendor/k8s.io/client-go/tools/cache",
        "//vendor/k8s.io/client-go/tools/clientcmd/api",
        "//vendor/k8s.io/client-go/util/workqueue",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
