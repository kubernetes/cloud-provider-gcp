load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "scale",
    srcs = ["scale.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/kubectl/pkg/scale",
    importpath = "k8s.io/kubectl/pkg/scale",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/autoscaling/v1:autoscaling",
        "//vendor/k8s.io/apimachinery/pkg/api/errors",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
        "//vendor/k8s.io/apimachinery/pkg/types",
        "//vendor/k8s.io/apimachinery/pkg/util/json",
        "//vendor/k8s.io/apimachinery/pkg/util/wait",
        "//vendor/k8s.io/client-go/scale",
    ],
)
