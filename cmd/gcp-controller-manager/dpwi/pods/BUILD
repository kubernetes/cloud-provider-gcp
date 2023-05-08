load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "pods",
    srcs = ["pods.go"],
    importpath = "k8s.io/cloud-provider-gcp/cmd/gcp-controller-manager/dpwi/pods",
    visibility = ["//visibility:public"],
    deps = [
        "//cmd/gcp-controller-manager/dpwi/ctxlog",
        "//cmd/gcp-controller-manager/dpwi/eventhandler",
        "//cmd/gcp-controller-manager/dpwi/serviceaccounts",
        "//vendor/k8s.io/api/core/v1:core",
        "//vendor/k8s.io/client-go/tools/cache",
    ],
)

go_test(
    name = "pods_test",
    srcs = ["pods_test.go"],
    embed = [":pods"],
    deps = [
        "//cmd/gcp-controller-manager/dpwi/serviceaccounts",
        "//vendor/k8s.io/api/core/v1:core",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/client-go/tools/cache",
    ],
)
