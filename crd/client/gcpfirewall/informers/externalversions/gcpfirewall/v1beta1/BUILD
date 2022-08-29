load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "gcpfirewall.go",
        "interface.go",
    ],
    importpath = "k8s.io/cloud-provider-gcp/crd/client/gcpfirewall/informers/externalversions/gcpfirewall/v1beta1",
    visibility = ["//visibility:public"],
    deps = [
        "//crd/apis/gcpfirewall/v1beta1:go_default_library",
        "//crd/client/gcpfirewall/clientset/versioned:go_default_library",
        "//crd/client/gcpfirewall/informers/externalversions/internalinterfaces:go_default_library",
        "//crd/client/gcpfirewall/listers/gcpfirewall/v1beta1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/runtime:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/watch:go_default_library",
        "//vendor/k8s.io/client-go/tools/cache:go_default_library",
    ],
)
