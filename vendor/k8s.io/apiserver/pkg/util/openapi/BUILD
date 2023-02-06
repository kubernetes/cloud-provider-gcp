load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "openapi",
    srcs = [
        "enablement.go",
        "proto.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/util/openapi",
    importpath = "k8s.io/apiserver/pkg/util/openapi",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/google/gnostic/openapiv2",
        "//vendor/k8s.io/apiserver/pkg/features",
        "//vendor/k8s.io/apiserver/pkg/util/feature",
        "//vendor/k8s.io/kube-openapi/pkg/common",
        "//vendor/k8s.io/kube-openapi/pkg/schemamutation",
        "//vendor/k8s.io/kube-openapi/pkg/util/proto",
        "//vendor/k8s.io/kube-openapi/pkg/validation/spec",
    ],
)
