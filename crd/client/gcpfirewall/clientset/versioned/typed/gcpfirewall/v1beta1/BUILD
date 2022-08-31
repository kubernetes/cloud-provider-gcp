load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "doc.go",
        "gcpfirewall.go",
        "gcpfirewall_client.go",
        "generated_expansion.go",
    ],
    importpath = "k8s.io/cloud-provider-gcp/crd/client/gcpfirewall/clientset/versioned/typed/gcpfirewall/v1beta1",
    visibility = ["//visibility:public"],
    deps = [
        "//crd/apis/gcpfirewall/v1beta1:go_default_library",
        "//crd/client/gcpfirewall/clientset/versioned/scheme:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/types:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/watch:go_default_library",
        "//vendor/k8s.io/client-go/rest:go_default_library",
    ],
)
