load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "policy",
    srcs = [
        "checker.go",
        "reader.go",
        "util.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/audit/policy",
    importpath = "k8s.io/apiserver/pkg/audit/policy",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
        "//vendor/k8s.io/apimachinery/pkg/util/sets",
        "//vendor/k8s.io/apiserver/pkg/apis/audit",
        "//vendor/k8s.io/apiserver/pkg/apis/audit/v1:audit",
        "//vendor/k8s.io/apiserver/pkg/apis/audit/validation",
        "//vendor/k8s.io/apiserver/pkg/audit",
        "//vendor/k8s.io/apiserver/pkg/authorization/authorizer",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
