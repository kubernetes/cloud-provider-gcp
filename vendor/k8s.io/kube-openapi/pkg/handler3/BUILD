load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "handler3",
    srcs = ["handler.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/kube-openapi/pkg/handler3",
    importpath = "k8s.io/kube-openapi/pkg/handler3",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/golang/protobuf/proto",
        "//vendor/github.com/google/gnostic/openapiv3",
        "//vendor/github.com/munnerz/goautoneg",
        "//vendor/k8s.io/kube-openapi/pkg/common",
        "//vendor/k8s.io/kube-openapi/pkg/internal/handler",
        "//vendor/k8s.io/kube-openapi/pkg/spec3",
        "//vendor/k8s.io/kube-openapi/pkg/validation/spec",
    ],
)
