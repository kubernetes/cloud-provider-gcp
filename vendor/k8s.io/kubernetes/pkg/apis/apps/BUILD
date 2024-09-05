load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "apps",
    srcs = [
        "doc.go",
        "register.go",
        "types.go",
        "zz_generated.deepcopy.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/kubernetes/pkg/apis/apps",
    importpath = "k8s.io/kubernetes/pkg/apis/apps",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
        "//vendor/k8s.io/apimachinery/pkg/util/intstr",
        "//vendor/k8s.io/kubernetes/pkg/apis/autoscaling",
        "//vendor/k8s.io/kubernetes/pkg/apis/core",
    ],
)
