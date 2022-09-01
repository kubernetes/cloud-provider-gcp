load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "builder",
    srcs = [
        "doc.go",
        "openapi.go",
        "util.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/kube-openapi/pkg/builder",
    importpath = "k8s.io/kube-openapi/pkg/builder",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/emicklei/go-restful/v3:go-restful",
        "//vendor/k8s.io/kube-openapi/pkg/common",
        "//vendor/k8s.io/kube-openapi/pkg/common/restfuladapter",
        "//vendor/k8s.io/kube-openapi/pkg/util",
        "//vendor/k8s.io/kube-openapi/pkg/validation/spec",
    ],
)
