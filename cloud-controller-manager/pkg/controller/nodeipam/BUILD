load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "nodeipam",
    srcs = [
        "doc.go",
        "legacyprovider.go",
        "node_ipam_controller.go",
    ],
    importpath = "k8s.io/cloud-provider-gcp/cloud-controller-manager/pkg/controller/nodeipam",
    visibility = ["//visibility:public"],
    deps = [
        "//cloud-controller-manager/pkg/controller/nodeipam/ipam",
        "//cloud-controller-manager/pkg/controller/nodeipam/ipam/sync",
        "//vendor/k8s.io/apimachinery/pkg/util/runtime",
        "//vendor/k8s.io/client-go/informers/core/v1:core",
        "//vendor/k8s.io/client-go/kubernetes",
        "//vendor/k8s.io/client-go/kubernetes/typed/core/v1:core",
        "//vendor/k8s.io/client-go/listers/core/v1:core",
        "//vendor/k8s.io/client-go/tools/cache",
        "//vendor/k8s.io/client-go/tools/record",
        "//vendor/k8s.io/cloud-provider",
        "//vendor/k8s.io/cloud-provider-gcp/crd/client/network/informers/externalversions/network/v1:network",
        "//vendor/k8s.io/component-base/metrics/prometheus/controllers",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)

go_test(
    name = "nodeipam_test",
    srcs = ["node_ipam_controller_test.go"],
    embed = [":nodeipam"],
    deps = [
        "//cloud-controller-manager/pkg/controller/nodeipam/ipam",
        "//cloud-controller-manager/pkg/controller/testutil",
        "//providers/gce",
        "//vendor/k8s.io/api/core/v1:core",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/client-go/informers",
        "//vendor/k8s.io/client-go/kubernetes/fake",
        "//vendor/k8s.io/cloud-provider-gcp/crd/client/network/clientset/versioned/fake",
        "//vendor/k8s.io/cloud-provider-gcp/crd/client/network/informers/externalversions",
        "//vendor/k8s.io/utils/net",
    ],
)
