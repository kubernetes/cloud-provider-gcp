load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = ["csrapproval.go"],
    importpath = "k8s.io/cloud-provider-gcp/pkg/csrapproval",
    visibility = ["//visibility:public"],
    deps = [
        "//pkg/csrmetrics:go_default_library",
        "//vendor/k8s.io/api/authorization/v1:go_default_library",
        "//vendor/k8s.io/api/certificates/v1beta1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//vendor/k8s.io/client-go/kubernetes:go_default_library",
        "//vendor/k8s.io/klog/v2:go_default_library",
        "//vendor/k8s.io/kubernetes/pkg/apis/certificates/v1beta1:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = ["csrapproval_test.go"],
    embed = [":go_default_library"],
    deps = [
        "//vendor/k8s.io/api/authorization/v1:go_default_library",
        "//vendor/k8s.io/api/certificates/v1beta1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/runtime:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema:go_default_library",
        "//vendor/k8s.io/client-go/kubernetes/fake:go_default_library",
        "//vendor/k8s.io/client-go/testing:go_default_library",
        "//vendor/k8s.io/klog/v2:go_default_library",
    ],
)
