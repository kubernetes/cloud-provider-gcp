load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "matchconditions",
    srcs = [
        "interface.go",
        "matcher.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/admission/plugin/webhook/matchconditions",
    importpath = "k8s.io/apiserver/pkg/admission/plugin/webhook/matchconditions",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/google/cel-go/cel",
        "//vendor/github.com/google/cel-go/common/types",
        "//vendor/k8s.io/api/admissionregistration/v1:admissionregistration",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/util/errors",
        "//vendor/k8s.io/apiserver/pkg/admission",
        "//vendor/k8s.io/apiserver/pkg/admission/metrics",
        "//vendor/k8s.io/apiserver/pkg/admission/plugin/cel",
        "//vendor/k8s.io/apiserver/pkg/apis/cel",
        "//vendor/k8s.io/apiserver/pkg/authorization/authorizer",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
