load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "v1beta2",
    srcs = [
        "controllerrevision.go",
        "daemonset.go",
        "daemonset_expansion.go",
        "deployment.go",
        "expansion_generated.go",
        "replicaset.go",
        "replicaset_expansion.go",
        "statefulset.go",
        "statefulset_expansion.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/client-go/listers/apps/v1beta2",
    importpath = "k8s.io/client-go/listers/apps/v1beta2",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/apps/v1beta2",
        "//vendor/k8s.io/api/core/v1:core",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/labels",
        "//vendor/k8s.io/client-go/listers",
        "//vendor/k8s.io/client-go/tools/cache",
    ],
)
