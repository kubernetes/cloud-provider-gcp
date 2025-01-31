load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "rbac",
    srcs = [
        "clusterrole.go",
        "clusterrolebinding.go",
        "expansion_generated.go",
        "role.go",
        "rolebinding.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/client-go/listers/rbac/v1",
    importpath = "k8s.io/client-go/listers/rbac/v1",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/rbac/v1:rbac",
        "//vendor/k8s.io/apimachinery/pkg/labels",
        "//vendor/k8s.io/client-go/listers",
        "//vendor/k8s.io/client-go/tools/cache",
    ],
)
