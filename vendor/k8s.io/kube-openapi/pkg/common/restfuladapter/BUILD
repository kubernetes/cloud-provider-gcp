load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "restfuladapter",
    srcs = [
        "adapter.go",
        "param_adapter.go",
        "response_error_adapter.go",
        "route_adapter.go",
        "webservice_adapter.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/kube-openapi/pkg/common/restfuladapter",
    importpath = "k8s.io/kube-openapi/pkg/common/restfuladapter",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/emicklei/go-restful/v3:go-restful",
        "//vendor/k8s.io/kube-openapi/pkg/common",
    ],
)
