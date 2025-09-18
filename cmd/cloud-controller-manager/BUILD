package(default_visibility = ["//visibility:public"])

load(
    "@io_bazel_rules_go//go:def.bzl",
    "go_binary",
    "go_library",
    "go_test",
)
load("//defs:version.bzl", "version_x_defs")

go_binary(
    name = "cloud-controller-manager",
    embed = [":cloud-controller-manager_lib"],
    pure = "on",
    x_defs = version_x_defs(),
)

go_library(
    name = "cloud-controller-manager_lib",
    srcs = [
        "gkenetworkparamsetcontroller.go",
        "gkeservicecontroller.go",
        "main.go",
        "nodeipamcontroller.go",
    ],
    importpath = "k8s.io/cloud-provider-gcp/cmd/cloud-controller-manager",
    deps = [
        "//cmd/cloud-controller-manager/options",
        "//pkg/controller/gkenetworkparamset",
        "//pkg/controller/nodeipam",
        "//pkg/controller/nodeipam/config",
        "//pkg/controller/nodeipam/ipam",
        "//pkg/controller/service",
        "//providers/gce",
        "//vendor/github.com/GoogleCloudPlatform/gke-networking-api/client/network/clientset/versioned",
        "//vendor/github.com/GoogleCloudPlatform/gke-networking-api/client/network/informers/externalversions",
        "//vendor/github.com/GoogleCloudPlatform/gke-networking-api/client/nodetopology/clientset/versioned",
        "//vendor/github.com/spf13/pflag",
        "//vendor/k8s.io/apimachinery/pkg/util/wait",
        "//vendor/k8s.io/apiserver/pkg/util/feature",
        "//vendor/k8s.io/cloud-provider",
        "//vendor/k8s.io/cloud-provider/app",
        "//vendor/k8s.io/cloud-provider/app/config",
        "//vendor/k8s.io/cloud-provider/names",
        "//vendor/k8s.io/cloud-provider/options",
        "//vendor/k8s.io/component-base/cli/flag",
        "//vendor/k8s.io/component-base/logs",
        "//vendor/k8s.io/component-base/metrics/prometheus/clientgo",
        "//vendor/k8s.io/component-base/metrics/prometheus/version",
        "//vendor/k8s.io/controller-manager/app",
        "//vendor/k8s.io/controller-manager/controller",
        "//vendor/k8s.io/klog/v2:klog",
        "//vendor/k8s.io/kubernetes/cmd/kube-controller-manager/names",
        "//vendor/k8s.io/utils/net",
    ],
)

load("//defs:container.bzl", "image")

image(binary = ":cloud-controller-manager")

go_test(
    name = "cloud-controller-manager_test",
    srcs = [
        "gkeservicecontroller_test.go",
        "nodeipamcontroller_test.go",
    ],
    embed = [":cloud-controller-manager_lib"],
    deps = [
        "//pkg/controller/nodeipam/config",
        "//pkg/controller/service",
        "//vendor/k8s.io/api/core/v1:core",
        "//vendor/k8s.io/cloud-provider",
        "//vendor/k8s.io/cloud-provider/app/config",
        "//vendor/k8s.io/cloud-provider/config",
        "//vendor/k8s.io/controller-manager/app",
    ],
)
