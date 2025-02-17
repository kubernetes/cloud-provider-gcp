load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "pager",
    srcs = ["pager.go"],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/client-go/tools/pager",
    importpath = "k8s.io/client-go/tools/pager",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/api/errors",
        "//vendor/k8s.io/apimachinery/pkg/api/meta",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/internalversion",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/util/runtime",
    ],
)
