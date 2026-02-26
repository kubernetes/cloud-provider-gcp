load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "providerconfigcr",
    srcs = [
        "finalizer.go",
        "informer.go",
        "providerconfig.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/providerconfigcr",
    importpath = "github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/providerconfigcr",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/GoogleCloudPlatform/gke-enterprise-mt/apis/providerconfig/v1:providerconfig",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1/unstructured",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
        "//vendor/k8s.io/client-go/dynamic",
        "//vendor/k8s.io/client-go/dynamic/dynamicinformer",
        "//vendor/k8s.io/client-go/tools/cache",
        "//vendor/k8s.io/client-go/util/retry",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
