load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "versioned",
    srcs = [
        "clientset.go",
        "doc.go",
    ],
    importpath = "k8s.io/cloud-provider-gcp/crd/client/gcpfirewall/clientset/versioned",
    visibility = ["//visibility:public"],
    deps = [
        "//crd/client/gcpfirewall/clientset/versioned/typed/gcpfirewall/v1beta1",
        "//vendor/k8s.io/client-go/discovery",
        "//vendor/k8s.io/client-go/rest",
        "//vendor/k8s.io/client-go/util/flowcontrol",
    ],
)
