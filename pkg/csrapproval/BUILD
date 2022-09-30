load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "csrapproval",
    srcs = ["csrapproval.go"],
    importpath = "k8s.io/cloud-provider-gcp/pkg/csrapproval",
    visibility = ["//visibility:public"],
    deps = [
        "//pkg/csrmetrics",
        "//vendor/k8s.io/api/authorization/v1:authorization",
        "//vendor/k8s.io/api/certificates/v1:certificates",
        "//vendor/k8s.io/api/core/v1:core",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/client-go/kubernetes",
        "//vendor/k8s.io/klog/v2:klog",
        "//vendor/k8s.io/kubernetes/pkg/apis/certificates/v1:certificates",
    ],
)

go_test(
    name = "csrapproval_test",
    srcs = ["csrapproval_test.go"],
    embed = [":csrapproval"],
    deps = [
        "//vendor/k8s.io/api/authorization/v1:authorization",
        "//vendor/k8s.io/api/certificates/v1:certificates",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
        "//vendor/k8s.io/client-go/kubernetes/fake",
        "//vendor/k8s.io/client-go/testing",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
