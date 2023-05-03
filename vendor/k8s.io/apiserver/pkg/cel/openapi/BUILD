load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "openapi",
    srcs = [
        "adaptor.go",
        "extensions.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/cel/openapi",
    importpath = "k8s.io/apiserver/pkg/cel/openapi",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/google/cel-go/common/types/ref",
        "//vendor/k8s.io/apimachinery/pkg/util/intstr",
        "//vendor/k8s.io/apiserver/pkg/cel",
        "//vendor/k8s.io/apiserver/pkg/cel/common",
        "//vendor/k8s.io/kube-openapi/pkg/validation/spec",
    ],
)
