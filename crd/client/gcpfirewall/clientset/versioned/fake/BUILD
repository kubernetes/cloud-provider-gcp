load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "clientset_generated.go",
        "doc.go",
        "register.go",
    ],
    importpath = "k8s.io/cloud-provider-gcp/crd/client/gcpfirewall/clientset/versioned/fake",
    visibility = ["//visibility:public"],
    deps = [
        "//crd/apis/gcpfirewall/v1alpha1:go_default_library",
        "//crd/client/gcpfirewall/clientset/versioned:go_default_library",
        "//crd/client/gcpfirewall/clientset/versioned/typed/gcpfirewall/v1alpha1:go_default_library",
        "//crd/client/gcpfirewall/clientset/versioned/typed/gcpfirewall/v1alpha1/fake:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/runtime:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/runtime/serializer:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/runtime:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/watch:go_default_library",
        "//vendor/k8s.io/client-go/discovery:go_default_library",
        "//vendor/k8s.io/client-go/discovery/fake:go_default_library",
        "//vendor/k8s.io/client-go/testing:go_default_library",
    ],
)
