load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "validation",
    srcs = ["validation.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/apis/apiserver/validation",
    importpath = "k8s.io/apiserver/pkg/apis/apiserver/validation",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/authorization/v1:authorization",
        "//vendor/k8s.io/api/authorization/v1beta1",
        "//vendor/k8s.io/apimachinery/pkg/util/sets",
        "//vendor/k8s.io/apimachinery/pkg/util/validation",
        "//vendor/k8s.io/apimachinery/pkg/util/validation/field",
        "//vendor/k8s.io/apiserver/pkg/apis/apiserver",
        "//vendor/k8s.io/apiserver/pkg/authentication/cel",
        "//vendor/k8s.io/apiserver/pkg/authorization/cel",
        "//vendor/k8s.io/apiserver/pkg/cel",
        "//vendor/k8s.io/apiserver/pkg/cel/environment",
        "//vendor/k8s.io/apiserver/pkg/features",
        "//vendor/k8s.io/apiserver/pkg/util/feature",
        "//vendor/k8s.io/client-go/util/cert",
    ],
)
