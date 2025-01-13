load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "nodeipam",
    srcs = [
        "doc.go",
        "legacyprovider.go",
        "node_ipam_controller.go",
    ],
    importpath = "k8s.io/cloud-provider-gcp/pkg/controller/nodeipam",
    visibility = ["//visibility:public"],
    deps = [
        "//pkg/controller/nodeipam/ipam",
        "//pkg/controller/nodeipam/ipam/sync",
        "//vendor/github.com/GoogleCloudPlatform/gke-networking-api/apis/nodetopology/v1:nodetopology",
        "//vendor/github.com/GoogleCloudPlatform/gke-networking-api/client/network/informers/externalversions/network/v1:network",
        "//vendor/github.com/GoogleCloudPlatform/gke-networking-api/client/nodetopology/clientset/versioned",
        "//vendor/github.com/GoogleCloudPlatform/gke-networking-api/client/nodetopology/clientset/versioned/typed/nodetopology/v1:nodetopology",
        "//vendor/k8s.io/apimachinery/pkg/util/runtime",
        "//vendor/k8s.io/client-go/informers/core/v1:core",
        "//vendor/k8s.io/client-go/kubernetes",
        "//vendor/k8s.io/client-go/kubernetes/typed/core/v1:core",
        "//vendor/k8s.io/client-go/listers/core/v1:core",
        "//vendor/k8s.io/client-go/tools/cache",
        "//vendor/k8s.io/client-go/tools/record",
        "//vendor/k8s.io/cloud-provider",
        "//vendor/k8s.io/component-base/metrics/prometheus/controllers",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)

go_test(
    name = "nodeipam_test",
    srcs = ["node_ipam_controller_test.go"],
    embed = [":nodeipam"],
    deps = [
        "//pkg/controller/nodeipam/ipam",
        "//pkg/controller/testutil",
        "//providers/gce",
        "//vendor/github.com/GoogleCloudPlatform/gke-networking-api/client/network/clientset/versioned/fake",
        "//vendor/github.com/GoogleCloudPlatform/gke-networking-api/client/network/informers/externalversions",
        "//vendor/k8s.io/api/core/v1:core",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/client-go/informers",
        "//vendor/k8s.io/client-go/kubernetes/fake",
        "//vendor/k8s.io/utils/net",
    ],
)
