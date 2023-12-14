load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "matching",
    srcs = ["matching.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/admission/plugin/validatingadmissionpolicy/matching",
    importpath = "k8s.io/apiserver/pkg/admission/plugin/validatingadmissionpolicy/matching",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/admissionregistration/v1:admissionregistration",
        "//vendor/k8s.io/api/admissionregistration/v1beta1",
        "//vendor/k8s.io/api/core/v1:core",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
        "//vendor/k8s.io/apiserver/pkg/admission",
        "//vendor/k8s.io/apiserver/pkg/admission/plugin/webhook/predicates/namespace",
        "//vendor/k8s.io/apiserver/pkg/admission/plugin/webhook/predicates/object",
        "//vendor/k8s.io/apiserver/pkg/admission/plugin/webhook/predicates/rules",
        "//vendor/k8s.io/client-go/kubernetes",
        "//vendor/k8s.io/client-go/listers/core/v1:core",
    ],
)
