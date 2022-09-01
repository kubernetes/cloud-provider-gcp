load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "memory",
    srcs = ["memcache.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/client-go/discovery/cached/memory",
    importpath = "k8s.io/client-go/discovery/cached/memory",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/google/gnostic/openapiv2",
        "//vendor/k8s.io/apimachinery/pkg/api/errors",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/util/runtime",
        "//vendor/k8s.io/apimachinery/pkg/version",
        "//vendor/k8s.io/client-go/discovery",
        "//vendor/k8s.io/client-go/openapi",
        "//vendor/k8s.io/client-go/openapi/cached",
        "//vendor/k8s.io/client-go/rest",
    ],
)
