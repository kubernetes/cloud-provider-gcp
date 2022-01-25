load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "atmostevery.go",
        "conflict.go",
        "fields.go",
        "managedfields.go",
        "pathelement.go",
    ],
    importmap = "k8s.io/cloud-provider-gcp/vendor/k8s.io/apiserver/pkg/endpoints/handlers/fieldmanager/internal",
    importpath = "k8s.io/apiserver/pkg/endpoints/handlers/fieldmanager/internal",
    visibility = ["//vendor/k8s.io/apiserver/pkg/endpoints/handlers/fieldmanager:__subpackages__"],
    deps = [
        "//vendor/k8s.io/apimachinery/pkg/api/errors:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/api/meta:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/apis/meta/v1:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/runtime:go_default_library",
        "//vendor/sigs.k8s.io/structured-merge-diff/v4/fieldpath:go_default_library",
        "//vendor/sigs.k8s.io/structured-merge-diff/v4/merge:go_default_library",
        "//vendor/sigs.k8s.io/structured-merge-diff/v4/value:go_default_library",
    ],
)
