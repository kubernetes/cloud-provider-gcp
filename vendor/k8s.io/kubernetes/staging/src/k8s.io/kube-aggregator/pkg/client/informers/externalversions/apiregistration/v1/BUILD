load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "apiservice.go",
        "interface.go",
    ],
    importmap = "io_k8s_cloud_provider_gcp/vendor/k8s.io/kubernetes/staging/src/k8s.io/kube-aggregator/pkg/client/informers/externalversions/apiregistration/v1",
    importpath = "k8s.io/kubernetes/staging/src/k8s.io/kube-aggregator/pkg/client/informers/externalversions/apiregistration/v1",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/runtime:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/watch:go_default_library",
        "//vendor/k8s.io/client-go/tools/cache:go_default_library",
        "@io_k8s_kube_aggregator//pkg/apis/apiregistration/v1:go_default_library",
        "@io_k8s_kube_aggregator//pkg/client/clientset_generated/clientset:go_default_library",
        "@io_k8s_kube_aggregator//pkg/client/informers/externalversions/internalinterfaces:go_default_library",
        "@io_k8s_kube_aggregator//pkg/client/listers/apiregistration/v1:go_default_library",
    ],
)

filegroup(
    name = "package-srcs",
    srcs = glob(["**"]),
    tags = ["automanaged"],
    visibility = ["//visibility:private"],
)

filegroup(
    name = "all-srcs",
    srcs = [":package-srcs"],
    tags = ["automanaged"],
    visibility = ["//visibility:public"],
)
