load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "features",
    srcs = [
        "client_adapter.go",
        "kube_features.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/kubernetes/pkg/features",
    importpath = "k8s.io/kubernetes/pkg/features",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apiextensions-apiserver/pkg/features",
        "//vendor/k8s.io/apimachinery/pkg/util/runtime",
        "//vendor/k8s.io/apimachinery/pkg/util/version",
        "//vendor/k8s.io/apiserver/pkg/features",
        "//vendor/k8s.io/apiserver/pkg/util/feature",
        "//vendor/k8s.io/client-go/features",
        "//vendor/k8s.io/component-base/featuregate",
        "//vendor/k8s.io/component-base/zpages/features",
        "//vendor/k8s.io/controller-manager/pkg/features",
    ],
)
