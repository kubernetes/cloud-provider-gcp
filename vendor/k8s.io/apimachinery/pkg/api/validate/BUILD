load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "validate",
    srcs = [
        "common.go",
        "doc.go",
        "each.go",
        "enum.go",
        "equality.go",
        "immutable.go",
        "item.go",
        "limits.go",
        "required.go",
        "subfield.go",
        "testing.go",
        "union.go",
        "zeroorone.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apimachinery/pkg/api/validate",
    importpath = "k8s.io/apimachinery/pkg/api/validate",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/api/equality",
        "//vendor/k8s.io/apimachinery/pkg/api/operation",
        "//vendor/k8s.io/apimachinery/pkg/api/validate/constraints",
        "//vendor/k8s.io/apimachinery/pkg/api/validate/content",
        "//vendor/k8s.io/apimachinery/pkg/util/sets",
        "//vendor/k8s.io/apimachinery/pkg/util/validation/field",
    ],
)
