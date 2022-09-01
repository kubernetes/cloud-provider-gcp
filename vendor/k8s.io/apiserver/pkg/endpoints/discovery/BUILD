load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "discovery",
    srcs = [
        "addresses.go",
        "group.go",
        "legacy.go",
        "root.go",
        "storageversionhash.go",
        "util.go",
        "version.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/endpoints/discovery",
    importpath = "k8s.io/apiserver/pkg/endpoints/discovery",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/emicklei/go-restful/v3:go-restful",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
        "//vendor/k8s.io/apimachinery/pkg/util/net",
        "//vendor/k8s.io/apiserver/pkg/endpoints/handlers/negotiation",
        "//vendor/k8s.io/apiserver/pkg/endpoints/handlers/responsewriters",
        "//vendor/k8s.io/klog/v2:klog",
    ],
)
