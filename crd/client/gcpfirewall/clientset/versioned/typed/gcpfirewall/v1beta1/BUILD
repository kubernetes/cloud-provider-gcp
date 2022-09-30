load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "v1beta1",
    srcs = [
        "doc.go",
        "gcpfirewall.go",
        "gcpfirewall_client.go",
        "generated_expansion.go",
    ],
    importpath = "k8s.io/cloud-provider-gcp/crd/client/gcpfirewall/clientset/versioned/typed/gcpfirewall/v1beta1",
    visibility = ["//visibility:public"],
    deps = [
        "//crd/apis/gcpfirewall/v1beta1",
        "//crd/client/gcpfirewall/clientset/versioned/scheme",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/types",
        "//vendor/k8s.io/apimachinery/pkg/watch",
        "//vendor/k8s.io/client-go/rest",
    ],
)
