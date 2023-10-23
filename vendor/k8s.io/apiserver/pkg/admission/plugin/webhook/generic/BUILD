load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "generic",
    srcs = [
        "interfaces.go",
        "webhook.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/admission/plugin/webhook/generic",
    importpath = "k8s.io/apiserver/pkg/admission/plugin/webhook/generic",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/admission/v1:admission",
        "//vendor/k8s.io/api/admission/v1beta1",
        "//vendor/k8s.io/api/admissionregistration/v1:admissionregistration",
        "//vendor/k8s.io/apimachinery/pkg/api/errors",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
        "//vendor/k8s.io/apiserver/pkg/admission",
        "//vendor/k8s.io/apiserver/pkg/admission/initializer",
        "//vendor/k8s.io/apiserver/pkg/admission/metrics",
        "//vendor/k8s.io/apiserver/pkg/admission/plugin/cel",
        "//vendor/k8s.io/apiserver/pkg/admission/plugin/webhook",
        "//vendor/k8s.io/apiserver/pkg/admission/plugin/webhook/config",
        "//vendor/k8s.io/apiserver/pkg/admission/plugin/webhook/predicates/namespace",
        "//vendor/k8s.io/apiserver/pkg/admission/plugin/webhook/predicates/object",
        "//vendor/k8s.io/apiserver/pkg/admission/plugin/webhook/predicates/rules",
        "//vendor/k8s.io/apiserver/pkg/authorization/authorizer",
        "//vendor/k8s.io/apiserver/pkg/cel/environment",
        "//vendor/k8s.io/apiserver/pkg/util/webhook",
        "//vendor/k8s.io/client-go/informers",
        "//vendor/k8s.io/client-go/kubernetes",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
