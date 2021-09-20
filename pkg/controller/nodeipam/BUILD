load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "doc.go",
        "legacyprovider.go",
        "node_ipam_controller.go",
    ],
    importpath = "k8s.io/cloud-provider-gcp/pkg/controller/nodeipam",
    visibility = ["//visibility:public"],
    deps = [
        "//pkg/controller/nodeipam/ipam:go_default_library",
        "//pkg/controller/nodeipam/ipam/sync:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/runtime:go_default_library",
        "//vendor/k8s.io/client-go/informers/core/v1:go_default_library",
        "//vendor/k8s.io/client-go/kubernetes:go_default_library",
        "//vendor/k8s.io/client-go/kubernetes/typed/core/v1:go_default_library",
        "//vendor/k8s.io/client-go/listers/core/v1:go_default_library",
        "//vendor/k8s.io/client-go/tools/cache:go_default_library",
        "//vendor/k8s.io/client-go/tools/record:go_default_library",
        "//vendor/k8s.io/cloud-provider:go_default_library",
        "//vendor/k8s.io/component-base/metrics/prometheus/ratelimiter:go_default_library",
        "//vendor/k8s.io/klog/v2:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = ["node_ipam_controller_test.go"],
    embed = [":go_default_library"],
    deps = [
        "//pkg/controller/nodeipam/ipam:go_default_library",
        "//pkg/controller/testutil:go_default_library",
        "//providers/gce:go_default_library",
        "//vendor/k8s.io/api/core/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//vendor/k8s.io/client-go/informers:go_default_library",
        "//vendor/k8s.io/client-go/kubernetes/fake:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/controller:go_default_library",
        "//vendor/k8s.io/utils/net:go_default_library",
    ],
)
