load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "v1beta1",
    srcs = [
        "daemonset.go",
        "daemonset_expansion.go",
        "deployment.go",
        "expansion_generated.go",
        "ingress.go",
        "networkpolicy.go",
        "replicaset.go",
        "replicaset_expansion.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/client-go/listers/extensions/v1beta1",
    importpath = "k8s.io/client-go/listers/extensions/v1beta1",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/apps/v1beta1",
        "//vendor/k8s.io/api/core/v1:core",
        "//vendor/k8s.io/api/extensions/v1beta1",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/labels",
        "//vendor/k8s.io/client-go/listers",
        "//vendor/k8s.io/client-go/tools/cache",
    ],
)
