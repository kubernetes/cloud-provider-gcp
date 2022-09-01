load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "webhook",
    srcs = [
        "metrics.go",
        "webhook.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/plugin/pkg/authorizer/webhook",
    importpath = "k8s.io/apiserver/plugin/pkg/authorizer/webhook",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/authorization/v1:authorization",
        "//vendor/k8s.io/api/authorization/v1beta1",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
        "//vendor/k8s.io/apimachinery/pkg/util/cache",
        "//vendor/k8s.io/apimachinery/pkg/util/wait",
        "//vendor/k8s.io/apiserver/pkg/authentication/user",
        "//vendor/k8s.io/apiserver/pkg/authorization/authorizer",
        "//vendor/k8s.io/apiserver/pkg/util/webhook",
        "//vendor/k8s.io/client-go/kubernetes/scheme",
        "//vendor/k8s.io/client-go/kubernetes/typed/authorization/v1:authorization",
        "//vendor/k8s.io/client-go/rest",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
