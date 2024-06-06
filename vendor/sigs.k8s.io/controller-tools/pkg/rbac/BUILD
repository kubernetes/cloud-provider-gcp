load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "rbac",
    srcs = [
        "parser.go",
        "zz_generated.markerhelp.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/sigs.k8s.io/controller-tools/pkg/rbac",
    importpath = "sigs.k8s.io/controller-tools/pkg/rbac",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/api/rbac/v1:rbac",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:meta",
        "//vendor/sigs.k8s.io/controller-tools/pkg/genall",
        "//vendor/sigs.k8s.io/controller-tools/pkg/markers",
    ],
)
