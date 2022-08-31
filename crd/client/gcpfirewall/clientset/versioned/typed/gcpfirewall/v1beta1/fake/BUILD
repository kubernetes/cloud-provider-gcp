load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "doc.go",
        "fake_gcpfirewall.go",
        "fake_gcpfirewall_client.go",
    ],
    importpath = "k8s.io/cloud-provider-gcp/crd/client/gcpfirewall/clientset/versioned/typed/gcpfirewall/v1beta1/fake",
    visibility = ["//visibility:public"],
    deps = [
        "//crd/apis/gcpfirewall/v1beta1:go_default_library",
        "//crd/client/gcpfirewall/clientset/versioned/typed/gcpfirewall/v1beta1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/labels:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/types:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/watch:go_default_library",
        "//vendor/k8s.io/client-go/rest:go_default_library",
        "//vendor/k8s.io/client-go/testing:go_default_library",
    ],
)
