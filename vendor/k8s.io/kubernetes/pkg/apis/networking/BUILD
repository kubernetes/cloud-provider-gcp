load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "networking",
    srcs = [
        "doc.go",
        "register.go",
        "types.go",
        "zz_generated.deepcopy.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/kubernetes/pkg/apis/networking",
    importpath = "k8s.io/kubernetes/pkg/apis/networking",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
        "//vendor/k8s.io/apimachinery/pkg/util/intstr",
        "//vendor/k8s.io/kubernetes/pkg/apis/core",
    ],
)
