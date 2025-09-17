load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "v1alpha3",
    srcs = [
        "devicetaintrule.go",
        "interface.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/client-go/informers/resource/v1alpha3",
    importpath = "k8s.io/client-go/informers/resource/v1alpha3",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/resource/v1alpha3",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/watch",
        "//vendor/k8s.io/client-go/informers/internalinterfaces",
        "//vendor/k8s.io/client-go/kubernetes",
        "//vendor/k8s.io/client-go/listers/resource/v1alpha3",
        "//vendor/k8s.io/client-go/tools/cache",
    ],
)
