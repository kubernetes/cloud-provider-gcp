load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "builder3",
    srcs = [
        "openapi.go",
        "util.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/kube-openapi/pkg/builder3",
    importpath = "k8s.io/kube-openapi/pkg/builder3",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/emicklei/go-restful/v3:go-restful",
        "//vendor/k8s.io/kube-openapi/pkg/builder3/util",
        "//vendor/k8s.io/kube-openapi/pkg/common",
        "//vendor/k8s.io/kube-openapi/pkg/common/restfuladapter",
        "//vendor/k8s.io/kube-openapi/pkg/spec3",
        "//vendor/k8s.io/kube-openapi/pkg/util",
        "//vendor/k8s.io/kube-openapi/pkg/validation/spec",
    ],
)
