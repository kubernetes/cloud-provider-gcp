load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "handler",
    srcs = [
        "default_pruning.go",
        "handler.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/kube-openapi/pkg/handler",
    importpath = "k8s.io/kube-openapi/pkg/handler",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/NYTimes/gziphandler",
        "//vendor/github.com/emicklei/go-restful/v3:go-restful",
        "//vendor/github.com/golang/protobuf/proto",
        "//vendor/github.com/google/gnostic/openapiv2",
        "//vendor/github.com/munnerz/goautoneg",
        "//vendor/k8s.io/klog/v2:klog",
        "//vendor/k8s.io/kube-openapi/pkg/builder",
        "//vendor/k8s.io/kube-openapi/pkg/common",
        "//vendor/k8s.io/kube-openapi/pkg/common/restfuladapter",
        "//vendor/k8s.io/kube-openapi/pkg/internal/handler",
        "//vendor/k8s.io/kube-openapi/pkg/validation/spec",
    ],
)
