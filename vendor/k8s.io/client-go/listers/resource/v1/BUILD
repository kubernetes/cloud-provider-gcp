load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "resource",
    srcs = [
        "deviceclass.go",
        "expansion_generated.go",
        "resourceclaim.go",
        "resourceclaimtemplate.go",
        "resourceslice.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/client-go/listers/resource/v1",
    importpath = "k8s.io/client-go/listers/resource/v1",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/resource/v1:resource",
        "//vendor/k8s.io/apimachinery/pkg/labels",
        "//vendor/k8s.io/client-go/listers",
        "//vendor/k8s.io/client-go/tools/cache",
    ],
)
