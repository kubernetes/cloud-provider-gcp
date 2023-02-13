load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "webhook",
    srcs = ["accessors.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/admission/plugin/webhook",
    importpath = "k8s.io/apiserver/pkg/admission/plugin/webhook",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/admissionregistration/v1:admissionregistration",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/labels",
        "//vendor/k8s.io/apiserver/pkg/admission/plugin/webhook/predicates/namespace",
        "//vendor/k8s.io/apiserver/pkg/admission/plugin/webhook/predicates/object",
        "//vendor/k8s.io/apiserver/pkg/util/webhook",
        "//vendor/k8s.io/client-go/rest",
    ],
)
