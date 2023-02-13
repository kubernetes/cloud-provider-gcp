load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "validation",
    srcs = ["validation.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/apis/config/validation",
    importpath = "k8s.io/apiserver/pkg/apis/config/validation",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/util/sets",
        "//vendor/k8s.io/apimachinery/pkg/util/validation/field",
        "//vendor/k8s.io/apiserver/pkg/apis/config",
    ],
)
