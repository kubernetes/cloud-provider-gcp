load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "rules",
    srcs = ["rules.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/admission/plugin/webhook/predicates/rules",
    importpath = "k8s.io/apiserver/pkg/admission/plugin/webhook/predicates/rules",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/admissionregistration/v1:admissionregistration",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
        "//vendor/k8s.io/apiserver/pkg/admission",
    ],
)
