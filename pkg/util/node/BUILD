load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "node",
    srcs = ["node.go"],
    importpath = "k8s.io/cloud-provider-gcp/pkg/util/node",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/GoogleCloudPlatform/gke-networking-api/apis/network/v1:network",
        "//vendor/k8s.io/api/core/v1:core",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/types",
        "//vendor/k8s.io/client-go/kubernetes",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)

go_test(
    name = "node_test",
    srcs = ["node_test.go"],
    embed = [":node"],
    deps = [
        "//pkg/controller/testutil",
        "//vendor/github.com/GoogleCloudPlatform/gke-networking-api/apis/network/v1:network",
        "//vendor/github.com/google/go-cmp/cmp",
        "//vendor/k8s.io/api/core/v1:core",
        "//vendor/k8s.io/apimachinery/pkg/api/resource",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/client-go/kubernetes/fake",
    ],
)
