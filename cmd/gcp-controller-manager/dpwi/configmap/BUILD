load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "configmap",
    srcs = ["configmap.go"],
    importpath = "k8s.io/cloud-provider-gcp/cmd/gcp-controller-manager/dpwi/configmap",
    visibility = ["//visibility:public"],
    deps = [
        "//cmd/gcp-controller-manager/dpwi/ctxlog",
        "//cmd/gcp-controller-manager/dpwi/eventhandler",
        "//cmd/gcp-controller-manager/dpwi/serviceaccounts",
        "//vendor/k8s.io/api/core/v1:core",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/client-go/informers/core/v1:core",
        "//vendor/k8s.io/client-go/kubernetes",
        "//vendor/k8s.io/client-go/tools/cache",
    ],
)

go_test(
    name = "configmap_test",
    srcs = ["configmap_test.go"],
    embed = [":configmap"],
    deps = [
        "//cmd/gcp-controller-manager/dpwi/serviceaccounts",
        "//vendor/k8s.io/api/core/v1:core",
        "//vendor/k8s.io/apimachinery/pkg/api/errors",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/client-go/kubernetes",
        "//vendor/k8s.io/client-go/kubernetes/fake",
        "//vendor/k8s.io/client-go/tools/cache",
    ],
)
