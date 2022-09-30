load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "request",
    srcs = [
        "auditid.go",
        "context.go",
        "doc.go",
        "received_time.go",
        "requestinfo.go",
        "webhook_duration.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/endpoints/request",
    importpath = "k8s.io/apiserver/pkg/endpoints/request",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/api/validation/path",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/internalversion",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/internalversion/scheme",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/types",
        "//vendor/k8s.io/apimachinery/pkg/util/sets",
        "//vendor/k8s.io/apiserver/pkg/authentication/user",
        "//vendor/k8s.io/klog/v2:klog",
        "//vendor/k8s.io/utils/clock",
    ],
)
