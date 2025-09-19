load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "endpointslice",
    srcs = [
        "ports.go",
        "wait.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/kubernetes/test/e2e/framework/endpointslice",
    importpath = "k8s.io/kubernetes/test/e2e/framework/endpointslice",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/core/v1:core",
        "//vendor/k8s.io/api/discovery/v1:discovery",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/types",
        "//vendor/k8s.io/apimachinery/pkg/util/sets",
        "//vendor/k8s.io/apimachinery/pkg/util/wait",
        "//vendor/k8s.io/client-go/kubernetes",
        "//vendor/k8s.io/kubernetes/test/e2e/framework",
    ],
)
