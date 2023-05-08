load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "request",
    srcs = [
        "admissionreview.go",
        "doc.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/admission/plugin/webhook/request",
    importpath = "k8s.io/apiserver/pkg/admission/plugin/webhook/request",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/admission/v1:admission",
        "//vendor/k8s.io/api/admission/v1beta1",
        "//vendor/k8s.io/api/authentication/v1:authentication",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/types",
        "//vendor/k8s.io/apimachinery/pkg/util/uuid",
        "//vendor/k8s.io/apiserver/pkg/admission",
        "//vendor/k8s.io/apiserver/pkg/admission/plugin/webhook/generic",
    ],
)
