load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "v1beta1",
    srcs = [
        "gcpfirewall.go",
        "interface.go",
    ],
    importpath = "k8s.io/cloud-provider-gcp/crd/client/gcpfirewall/informers/externalversions/gcpfirewall/v1beta1",
    visibility = ["//visibility:public"],
    deps = [
        "//crd/apis/gcpfirewall/v1beta1",
        "//crd/client/gcpfirewall/clientset/versioned",
        "//crd/client/gcpfirewall/informers/externalversions/internalinterfaces",
        "//crd/client/gcpfirewall/listers/gcpfirewall/v1beta1",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/watch",
        "//vendor/k8s.io/client-go/tools/cache",
    ],
)
