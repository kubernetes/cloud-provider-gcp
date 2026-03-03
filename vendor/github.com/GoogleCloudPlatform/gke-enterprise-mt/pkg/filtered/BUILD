load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "filtered",
    srcs = [
        "filteredcache.go",
        "filteredfactory.go",
        "filteredinformer.go",
        "helpers.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/filtered",
    importpath = "github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/filtered",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/api/meta",
        "//vendor/k8s.io/client-go/informers",
        "//vendor/k8s.io/client-go/informers/coordination",
        "//vendor/k8s.io/client-go/informers/coordination/v1:coordination",
        "//vendor/k8s.io/client-go/informers/core",
        "//vendor/k8s.io/client-go/informers/core/v1:core",
        "//vendor/k8s.io/client-go/listers/coordination/v1:coordination",
        "//vendor/k8s.io/client-go/listers/core/v1:core",
        "//vendor/k8s.io/client-go/tools/cache",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
