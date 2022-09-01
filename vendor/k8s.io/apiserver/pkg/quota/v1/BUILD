load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "quota",
    srcs = [
        "interfaces.go",
        "resources.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/quota/v1",
    importpath = "k8s.io/apiserver/pkg/quota/v1",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/core/v1:core",
        "//vendor/k8s.io/apimachinery/pkg/api/resource",
        "//vendor/k8s.io/apimachinery/pkg/runtime",
        "//vendor/k8s.io/apimachinery/pkg/runtime/schema",
        "//vendor/k8s.io/apimachinery/pkg/util/errors",
        "//vendor/k8s.io/apimachinery/pkg/util/sets",
        "//vendor/k8s.io/apiserver/pkg/admission",
        "//vendor/k8s.io/client-go/tools/cache",
    ],
)
