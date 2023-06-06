load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "eventhandler",
    srcs = ["eventhandler.go"],
    importpath = "k8s.io/cloud-provider-gcp/cmd/gcp-controller-manager/dpwi/eventhandler",
    visibility = ["//visibility:public"],
    deps = [
        "//cmd/gcp-controller-manager/dpwi/ctxlog",
        "//vendor/github.com/google/uuid",
        "//vendor/k8s.io/apimachinery/pkg/util/wait",
        "//vendor/k8s.io/client-go/tools/cache",
        "//vendor/k8s.io/client-go/util/workqueue",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)

go_test(
    name = "eventhandler_test",
    srcs = ["eventhandler_test.go"],
    embed = [":eventhandler"],
    deps = [
        "//vendor/k8s.io/api/core/v1:core",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/client-go/tools/cache",
    ],
)
