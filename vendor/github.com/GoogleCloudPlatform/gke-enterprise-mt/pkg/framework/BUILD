load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "framework",
    srcs = [
        "controller.go",
        "controller_map.go",
        "manager.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/framework",
    importpath = "github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/framework",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/GoogleCloudPlatform/gke-enterprise-mt/apis/providerconfig/v1:providerconfig",
        "//vendor/github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/framework/mtcontext",
        "//vendor/github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/framework/taskqueue",
        "//vendor/github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/providerconfigcr",
        "//vendor/k8s.io/apimachinery/pkg/api/errors",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/client-go/dynamic",
        "//vendor/k8s.io/client-go/tools/cache",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
