load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "webhookadmission",
    srcs = [
        "doc.go",
        "register.go",
        "types.go",
        "zz_generated.deepcopy.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/admission/plugin/webhook/config/apis/webhookadmission",
    importpath = "k8s.io/apiserver/pkg/admission/plugin/webhook/config/apis/webhookadmission",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
    ],
)
