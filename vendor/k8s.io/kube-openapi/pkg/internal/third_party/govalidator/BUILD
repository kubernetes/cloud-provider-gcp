load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "govalidator",
    srcs = [
        "patterns.go",
        "validator.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/kube-openapi/pkg/internal/third_party/govalidator",
    importpath = "k8s.io/kube-openapi/pkg/internal/third_party/govalidator",
    visibility = ["//vendor/k8s.io/kube-openapi/pkg:__subpackages__"],
)
