load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "scheduling",
    srcs = [
        "expansion_generated.go",
        "priorityclass.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/client-go/listers/scheduling/v1",
    importpath = "k8s.io/client-go/listers/scheduling/v1",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/scheduling/v1:scheduling",
        "//vendor/k8s.io/apimachinery/pkg/labels",
        "//vendor/k8s.io/client-go/listers",
        "//vendor/k8s.io/client-go/tools/cache",
    ],
)
